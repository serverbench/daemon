package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	gonanoid "github.com/matoous/go-nanoid/v2"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
	"supervisor/client/action"
	"supervisor/client/proto"
	"supervisor/client/proto/pipe"
	"supervisor/containers"
	"supervisor/machine"
	"supervisor/machine/hardware"
	"time"
)

type ListenerContext struct {
	context context.Context
	cancel  context.CancelFunc
}

type Client struct {
	SendChan    chan proto.Msg
	ForwardChan chan pipe.Forward
	Id          *string
	Conn        *websocket.Conn
	Cli         *client.Client
	Machine     *machine.Machine
	callbacks   map[string]chan proto.Reply
	pipes       map[string]pipe.Pipe

	// Add connection monitoring fields
	pingInterval time.Duration
	pongWait     time.Duration
	writeWait    time.Duration
}

const responseTimeout = time.Second * 5

func (c *Client) sendRaw(action string, data map[string]interface{}) (string, error) {
	rid, err := gonanoid.New()
	if err != nil {
		return "", err
	}
	c.SendChan <- proto.Msg{
		Action: "machine." + action,
		Rid:    rid,
		Params: data,
	}
	return rid, nil
}

// SendAndWait sends a message and waits for a response (with timeout)
func (c *Client) SendAndWait(action string, data map[string]interface{}, result any) error {
	rid, err := c.sendRaw(action, data)
	if err != nil {
		return err
	}

	replyChan := make(chan proto.Reply, 1)
	c.callbacks[rid] = replyChan
	defer delete(c.callbacks, rid)

	select {
	case reply := <-replyChan:
		if reply.Result != nil {
			// Attempt to decode Params into the result
			responseBytes, err := json.Marshal(reply.Result)
			if err != nil {
				return err
			}

			err = json.Unmarshal(responseBytes, result)
			if err != nil {
				return err
			}
		}
		return nil
	case <-time.After(responseTimeout):
		return errors.New("timeout waiting for reply")
	}
}

func (c *Client) handleMessage(msg proto.Incoming) error {
	// You can switch on msg.Action here if you want
	switch *msg.Realm {
	case "machine":
		{
			switch *msg.Action {
			case "containers":
				{
					return c.containers()
				}
			case "actions":
				{
					return c.actions()
				}
			}
		}
	}
	return errors.New("unknown message")
}

func (c *Client) handleListener(listener pipe.Pipe) (err error) {
	log.Info("handling listener")
	var selectedContainer *containers.Container = nil
	jsonData, err := json.Marshal(listener.Filter)
	genericFilter := pipe.GenericFilter{}
	err = json.Unmarshal(jsonData, &genericFilter)
	if err == nil {
		for _, container := range c.Machine.Containers {
			if container.Id == genericFilter.Container {
				selectedContainer = &container
				break
			}
		}
	}
	log.Info("handling container")
	if selectedContainer != nil {
		switch listener.Event {
		case pipe.EventStatus:
			err = selectedContainer.PipeStatus(listener.Context, c.Cli, &listener)
			break
		case pipe.EventLog:
			logFilter := pipe.LogFilter{}
			err = json.Unmarshal(jsonData, &logFilter)
			if err != nil {
				err = errors.New("unknown log filter")
			} else {
				err = selectedContainer.PipeLogs(listener.Context, c.Cli, logFilter.Since, logFilter.Until, logFilter.Limit, &listener)
			}
			break
		case pipe.EventPassword:
			password, err := selectedContainer.ResetPassword()
			if err == nil {
				listener.Forward <- listener.Package(pipe.Password{
					Password: password,
				})
				listener.End()
			}
			break
		case pipe.EventGit:
			gitFilter := pipe.GitFilter{}
			err = json.Unmarshal(jsonData, &gitFilter)
			if err != nil {
				err = errors.New("unknown git filter")
			} else {
				err = selectedContainer.Pull(c.Cli, gitFilter.Token, gitFilter.Uri, gitFilter.Branch, gitFilter.Domain)
				if err == nil {
					listener.Forward <- listener.Package(pipe.Git{
						Deployed: true,
					})
				}
				listener.End()
			}
			break
		}

	} else {
		err = errors.New("container not found")
	}
	select {
	case <-listener.Delete:
		delete(c.pipes, listener.Lid)
	}
	if err != nil {
		log.Error(err)
		listener.End()
		return err
	}
	return nil
}

func (c *Client) containers() (err error) {
	updatedContainers := make([]containers.Container, 0)
	err = c.MachineSendAndWait("containers", map[string]interface{}{}, &updatedContainers)
	if err != nil {
		return err
	}
	created, err := c.Machine.UpdateContainers(c.Cli, updatedContainers)
	if err != nil {
		log.Error("update containers failed", err)
		return err
	}
	for _, container := range created {
		var x interface{}
		err = c.MachineSendAndWait("containers."+container.Id+".postcreate", map[string]interface{}{}, &x)
		if err != nil {
			log.Error("create container ack failed", err)
			return err
		}
	}
	for _, container := range c.Machine.Containers {
		if container.Branch != nil {
			commit, err := container.GetCommit()
			if err != nil {
				log.Error("get container commit failed", err)
				return err
			}
			ignore := make(map[string]struct{})
			data := make(map[string]interface{})
			data["commit"] = commit
			err = c.MachineSendAndWait("containers."+container.Id+".commit", data, &ignore)
			if err != nil {
				log.Error("commit container request failed", err)
				return err
			}
		}
	}
	return nil
}

func (c *Client) handshake() (err error) {
	// Immediately send hardware info
	var session machine.Session
	err = c.SendAndWait("session", map[string]interface{}{
		"key": c.Machine.Key,
	}, &session)
	if err != nil {
		log.Fatal(err)
		return err
	}
	c.Id = &session.Machine.Id
	log.Info("Connected with session id " + *c.Id)
	err = c.sendHardware()
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = c.containers()
	if err != nil {
		log.Fatal(err)
		return err
	}
	return err
}

func (c *Client) sendHardware() (err error) {
	hw, err := hardware.GetHardware(c.Cli)
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = c.MachineSendAndWait("update", map[string]interface{}{
		"hardware": hw,
	}, proto.Reply{})
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Info("Updated hardware")
	return nil
}

func (c *Client) MachineSendAndWait(action string, data map[string]interface{}, result any) (err error) {
	return c.SendAndWait(*c.Id+"."+action, data, result)
}

func (c *Client) ContainerSendAndWait(container containers.Container, action string, data map[string]interface{}, result any) (err error) {
	return c.MachineSendAndWait("container."+container.Id+"."+action, data, result)
}

// Add ping/pong handling to detect connection issues
func (c *Client) setupPingPong() {
	// Set up ping/pong handling
	c.Conn.SetPongHandler(func(string) error {
		log.Debug("Received pong")
		c.Conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})

	// Set initial read deadline
	c.Conn.SetReadDeadline(time.Now().Add(c.pongWait))
}

func (c *Client) pingHandler(done chan struct{}) {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Error("Failed to send ping:", err)
				return
			}
			log.Debug("Sent ping")
		case <-done:
			return
		}
	}
}

func (c *Client) Start(cli *client.Client) (err error) {
	c.Cli = cli
	c.pipes = make(map[string]pipe.Pipe)
	c.ForwardChan = make(chan pipe.Forward, 100)
	c.SendChan = make(chan proto.Msg, 100)
	c.callbacks = make(map[string]chan proto.Reply)

	// Set connection monitoring timeouts
	c.pingInterval = 30 * time.Second // Send ping every 30 seconds
	c.pongWait = 60 * time.Second     // Wait 60 seconds for pong response
	c.writeWait = 10 * time.Second    // Wait 10 seconds for write to complete

	c.Machine, err = machine.GetMachine(cli)
	if err != nil {
		return err
	}
	endpoint := os.Getenv("ENDPOINT")
	if endpoint == "" {
		endpoint = "wss://stream.beta.serverbench.io"
	}
	log.Info(fmt.Sprintf("Connecting to %s", endpoint))
	endpoint = endpoint + "/?key=" + c.Machine.Key
	u, err := url.Parse(endpoint)
	if err != nil {
		log.Error("error parsing endpoint url")
		return err
	}
	c.Conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	defer c.Conn.Close()

	// Set up ping/pong handling
	c.setupPingPong()

	// Create channels for coordinating goroutines
	done := make(chan struct{})
	writerDone := make(chan struct{})
	forwardDone := make(chan struct{})

	// Start ping handler
	go c.pingHandler(done)

	// Reader goroutine
	go func() {
		defer close(done) // Signal when the reader exits
		for {
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				log.Error("WebSocket read error:", err)
				return
			}
			var incoming proto.Incoming
			if err := json.Unmarshal(message, &incoming); err != nil {
				log.Error("failed to decode incoming:", err)
				continue
			}
			if incoming.Action != nil {
				go func() {
					err := c.handleMessage(incoming)
					if err != nil {
						log.Error("error handling message:", err)
					}
				}()
			} else if incoming.Lid != nil {
				if incoming.Close != nil && *incoming.Close == true {
					existing, ok := c.pipes[*incoming.Lid]
					if ok {
						existing.End()
					} else {
						err = errors.New("unknown lid")
						log.Error("error while closing listener:", err)
						continue
					}
				}
				var listener pipe.BasicPipe
				if err := json.Unmarshal(message, &listener); err != nil {
					log.Error("failed to decode listener:", err)
					continue
				}
				ctx, cancel := context.WithCancel(context.Background())
				completeListener := pipe.Pipe{
					Lid:     listener.Lid,
					Delete:  make(chan struct{}, 1),
					Cancel:  cancel,
					Context: ctx,
					Forward: c.ForwardChan,
					Event:   listener.Event,
					Filter:  listener.Filter,
				}
				c.pipes[completeListener.Lid] = completeListener
				go func() {
					err := c.handleListener(completeListener)
					if err != nil {
						log.Error("error handling listener:", err)
					}
				}()
			} else {
				var reply proto.Reply
				if err := json.Unmarshal(message, &reply); err != nil {
					log.Error("failed to decode reply:", err)
					continue
				}
				if reply.Rid != "" {
					if ch, ok := c.callbacks[reply.Rid]; ok {
						ch <- reply
						continue
					}
				}
			}
		}
	}()

	// Writer goroutine
	go func() {
		defer close(writerDone)
		for {
			select {
			case msg, ok := <-c.SendChan:
				if !ok {
					log.Info("SendChan closed, stopping writer")
					return
				}
				c.Conn.SetWriteDeadline(time.Now().Add(c.writeWait))
				err := c.Conn.WriteJSON(msg)
				if err != nil {
					log.Error("write error:", err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Forward goroutine
	go func() {
		defer close(forwardDone)
		for {
			select {
			case msg, ok := <-c.ForwardChan:
				if !ok {
					log.Info("ForwardChan closed, stopping forwarder")
					return
				}
				c.Conn.SetWriteDeadline(time.Now().Add(c.writeWait))
				err := c.Conn.WriteJSON(msg)
				if err != nil {
					log.Error("forward write error:", err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Perform handshake
	if err := c.handshake(); err != nil {
		return err
	}
	// Request queued actions and listen for new ones
	if err := c.actions(); err != nil {
		return err
	}

	// Wait for any goroutine to signal completion (indicating connection issues)
	select {
	case <-done:
		log.Info("Reader goroutine finished")
	case <-writerDone:
		log.Info("Writer goroutine finished")
	case <-forwardDone:
		log.Info("Forward goroutine finished")
	}

	// Clean up - close channels to stop other goroutines
	close(c.SendChan)
	close(c.ForwardChan)

	// Wait a bit for goroutines to finish
	time.Sleep(time.Second)

	log.Info("WebSocket connection closed, exiting")
	return errors.New("websocket connection closed")
}

func (c *Client) actions() error {
	var rawMessages []json.RawMessage
	if err := c.MachineSendAndWait("actions", map[string]interface{}{}, &rawMessages); err != nil {
		return err
	}

	for _, raw := range rawMessages {
		var a action.Action
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("failed to unmarshal action header: %w", err)
		}
		a.Ref = raw
		update, actionErr := a.Process(c.Cli)
		if actionErr != nil {
			a.Ref = nil
			log.Error("error processing action", a, actionErr)
		}
		for i := range c.Machine.Containers {
			if c.Machine.Containers[i].Id == a.Container.Id {
				c.Machine.Containers[i].ExpectingFirstCommit = false
				c.Machine.Containers[i].Ports = a.Container.Ports
				c.Machine.Containers[i].Image = a.Container.Image
				c.Machine.Containers[i].Branch = a.Container.Branch
				c.Machine.Containers[i].Envs = a.Container.Envs
				c.Machine.Containers[i].Mount = a.Container.Mount
			}
		}
		if update != nil {
			actionErr = c.ContainerSendAndWait(a.Container, update.Action, update.Params, &proto.Reply{})
			if actionErr != nil {
				a.Ref = nil
				log.Error("error processing action trigger", a, actionErr)
			}
		}
	}

	return nil
}
