package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	gonanoid "github.com/matoous/go-nanoid/v2"
	log "github.com/sirupsen/logrus"
	"net/url"
	"supervisor/client/proto"
	"supervisor/machine"
	"supervisor/machine/hardware"
	"time"
)

type Client struct {
	SendChan  chan proto.Msg
	Id        *string
	Conn      *websocket.Conn
	Cli       *client.Client
	Machine   *machine.Machine
	callbacks map[string]chan proto.Reply
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

func (c *Client) handleMessage(msg proto.Msg) {
	log.Infof("Handling action %s with params %+v", msg.Action, msg.Params)
	// You can switch on msg.Action here if you want
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
	hw, err := hardware.GetHardware(c.Cli)
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = c.SendAndWait(*c.Id+".update", map[string]interface{}{
		"hardware": hw,
	}, proto.Reply{})
	if err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func (c *Client) Start(cli *client.Client) (err error) {
	c.Cli = cli
	c.SendChan = make(chan proto.Msg, 100)
	c.callbacks = make(map[string]chan proto.Reply)
	c.Machine, err = machine.GetMachine(cli)
	if err != nil {
		return err
	}

	u := url.URL{Scheme: "wss", Host: "stream.beta.serverbench.io", Path: "/", RawQuery: "key=" + c.Machine.Key}
	log.Info(fmt.Sprintf("connecting to %s", u.String()))
	c.Conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	defer c.Conn.Close()

	// Create a done channel
	done := make(chan struct{})

	// Reader goroutine
	go func() {
		defer close(done) // Signal when the reader exits
		for {
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			var incoming proto.Incoming
			if err := json.Unmarshal(message, &incoming); err != nil {
				log.Println("failed to decode incoming:", err)
				continue
			}
			if incoming.Action != nil {
				var msg proto.Msg
				if err := json.Unmarshal(message, &msg); err != nil {
					log.Println("failed to decode message:", err)
					continue
				}
				go c.handleMessage(msg)
			} else {
				var reply proto.Reply
				if err := json.Unmarshal(message, &reply); err != nil {
					log.Println("failed to decode reply:", err)
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
		for msg := range c.SendChan {
			err := c.Conn.WriteJSON(msg)
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}()

	// Perform handshake
	if err := c.handshake(); err != nil {
		return err
	}

	// Wait for the done channel to be closed (when the reader exits)
	<-done

	return nil
}
