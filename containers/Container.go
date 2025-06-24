package containers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"supervisor/client/proto/pipe"
	"supervisor/machine/hardware"
	"time"
)

var unknownContainer = errors.New("unknown container")

type Container struct {
	Id      string            `json:"id"`
	Image   string            `json:"image"`
	Address string            `json:"address"`
	Mount   string            `json:"mount"`
	Envs    map[string]string `json:"envs"`
	Ports   []Port            `json:"ports"`
}

func (c Container) PipeLogs(ctx context.Context, cli *client.Client, since int64, until int64, limit int64, listener *pipe.Pipe) (err error) {
	sinceStr := ""
	untilStr := ""
	follow := false
	if until > 0 {
		untilStr = time.Unix(until/1000, (until%1000)*1e6).UTC().Format(time.RFC3339)
	}
	if since > 0 {
		sinceStr = time.Unix(since/1000, (since%1000)*1e6).UTC().Format(time.RFC3339)
	}
	if until <= 0 {
		follow = true
	}

	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	reader, err := cli.ContainerLogs(ctx, cid, container.LogsOptions{
		Follow:     follow,
		Since:      sinceStr,
		Until:      untilStr,
		ShowStderr: true,
		ShowStdout: true,
		Timestamps: true,
	})
	if err != nil {
		return err
	}
	defer reader.Close()

	processed := int64(0)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) < 8 {
			continue // malformed frame
		}
		line = line[8:] // strip Docker's log header

		logLine := string(line)

		parts := strings.SplitN(logLine, " ", 2)
		if len(parts) != 2 {
			continue
		}

		timestampStr, content := parts[0], parts[1]
		timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
		if err != nil {
			continue
		}
		listener.Forward <- listener.Package(pipe.Log{
			Timestamp: timestamp.UnixMilli(),
			Content:   content,
			End:       false,
		})
		processed++
		if limit > 0 && processed >= limit {
			log.Info("cancelled")
			listener.Cancel()
			break
		}
	}

	// If not following, or once the reader ends, send a closing log
	listener.End()

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (c Container) PipeStatus(ctx context.Context, cli *client.Client, listener *pipe.Pipe) error {
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	// Step 1: Send initial status using ContainerInspect
	containerJSON, err := cli.ContainerInspect(ctx, cid)
	if err != nil {
		return err
	}

	initial := pipe.Status{
		Status: containerJSON.State.Status,
	}

	select {
	case listener.Forward <- listener.Package(initial):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Step 2: Set up event filter and follow event stream
	filterArgs := filters.NewArgs()
	filterArgs.Add("type", "container")
	filterArgs.Add("container", cid)

	eventChan, errChan := cli.Events(ctx, events.ListOptions{
		Filters: filterArgs,
	})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errChan:
			if err != nil {
				return err
			}
			return nil // closed without error

		case event := <-eventChan:
			if event.Type != "container" || event.Action == "" {
				continue
			}

			normalized, ok := pipe.NormalizeDockerStatus[event.Action]
			if !ok {
				continue
			}

			s := pipe.Status{
				Status: normalized,
			}

			select {
			case listener.Forward <- listener.Package(s):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (c Container) Dir() string {
	return filepath.Join("/containers/", c.Id)
}

func (c Container) HostDir(cli *client.Client) (hostPath *string, err error) {
	containerRoot, err := hardware.GetHostPath(cli)
	if err != nil {
		return hostPath, err
	}
	var finalPath = filepath.Join(*containerRoot, c.Id)
	return &finalPath, err
}

// Create creates the user and spins up the container
func (c Container) Create(cli *client.Client) (err error) {
	err = c.createUser()
	if err != nil {
		return err
	}
	return c.Update(cli)
}

// Update applies the new firewall rules and creates (or updates) the container
func (c Container) Update(cli *client.Client) (err error) {
	err = c.InstallFirewall()
	if err != nil {
		return err
	}
	err = c.pullImage(cli)
	if err != nil {
		return err
	}
	err = c.createContainer(cli)
	if err != nil {
		return err
	}
	return c.Start(cli)
}

func (c Container) InstallFirewall() (err error) {
	firewall, err := c.firewall(c.Ports)
	if err != nil {
		return err
	}
	err = firewall.Install()
	return err
}

func (c Container) pullImage(cli *client.Client) (err error) {
	log.Info("pulling image")
	out, err := cli.ImagePull(context.Background(), c.Image, image.PullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	// Stream output to the console
	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return fmt.Errorf("error reading image pull stream: %w", err)
	}
	return nil
}

func (c Container) createContainer(cli *client.Client) (err error) {
	_, fetchErr := c.cId(cli)
	if fetchErr == nil {
		err = c.Stop(cli)
		if err != nil {
			return err
		}
		err = c.deleteContainer(cli)
		if err != nil {
			return err
		}
	}
	log.Info("creating container")
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	for _, p := range c.Ports {
		for _, proto := range protocols {
			portStr := fmt.Sprintf("%d/%s", p.Port, proto)
			natPort := nat.Port(portStr)
			exposedPorts[natPort] = struct{}{}
			portBindings[natPort] = []nat.PortBinding{
				{
					HostIP:   c.Address,
					HostPort: fmt.Sprintf("%d", p.Port),
				},
			}
		}
	}

	var env []string
	for k, v := range c.Envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	config := &container.Config{
		Image:        c.Image,
		ExposedPorts: exposedPorts,
		Env:          env,
	}
	hostPath, err := c.HostDir(cli)
	if err != nil {
		return err
	}
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: *hostPath,
				Target: c.Mount,
			},
		},
	}
	_, err = cli.ContainerCreate(context.Background(), config, hostConfig, nil, nil, c.cName())
	if err != nil {
		return err
	}
	return err
}

func (c Container) Start(cli *client.Client) (err error) {
	log.Info("starting container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerStart(ctx, cid, container.StartOptions{})
}

func (c Container) Stop(cli *client.Client) (err error) {
	log.Info("stopping container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerStop(ctx, cid, container.StopOptions{})
}

func (c Container) Restart(cli *client.Client) (err error) {
	log.Info("restarting container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerRestart(ctx, cid, container.StopOptions{})
}

func (c Container) Pause(cli *client.Client) (err error) {
	log.Info("pausing container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerPause(ctx, cid)
}

func (c Container) Unpause(cli *client.Client) (err error) {
	log.Info("unpausing container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerUnpause(ctx, cid)
}

func (c Container) Kill(cli *client.Client) (err error) {
	log.Info("killing container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerKill(ctx, cid, "SIGKILL")
}

func (c Container) deleteContainer(cli *client.Client) (err error) {
	log.Info("deleting container")
	cid, err := c.cId(cli)
	if err != nil {
		if errors.Is(err, unknownContainer) {
			return nil
		}
		return err
	}
	return cli.ContainerRemove(context.Background(), cid, container.RemoveOptions{
		Force: true,
	})
}

func (c Container) cName() (cname string) {
	return "sb-" + c.Id
}

func (c Container) cId(cli *client.Client) (cid string, err error) {
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: true,
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "name",
			Value: "^/" + c.cName() + "$",
		}),
	})
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return cid, unknownContainer
	}
	return containers[0].ID, nil
}

// Destroy removes everything related to that container
func (c Container) Destroy(cli *client.Client) (err error) {
	err = c.deleteContainer(cli)
	if err != nil {
		return err
	}
	err = c.deleteUser()
	if err != nil {
		return err
	}
	err = c.Clear()
	if err != nil {
		return err
	}
	firewall, err := c.firewall(make([]Port, 0))
	if err != nil {
		return err
	}
	err = firewall.Uninstall()
	return err
}

func (c Container) Clear() error {
	log.Info("clearing data")
	return exec.Command("rm", "-rf", filepath.Join(c.Dir(), "**")).Run()
}
