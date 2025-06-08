package containers

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
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
	"supervisor/machine/hardware"
)

var unknownContainer = errors.New("unknown container")

type Container struct {
	Id      string            `json:"id"`
	Image   string            `json:"image"`
	Address string            `json:"address"`
	Mount   string            `json:"mount"`
	Env     map[string]string `json:"env"`
	Ports   []Port            `json:"ports"`
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
	err = c.MountDir()
	if err != nil {
		return err
	}
	return c.Update(cli)
}

// Update applies the new firewall rules and creates (or updates) the container
func (c Container) Update(cli *client.Client) (err error) {
	firewall, err := c.firewall(c.Ports)
	if err != nil {
		return err
	}
	err = firewall.Install()
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
	return c.startContainer(cli)
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

func (c Container) createContainer(cli *client.Client) error {
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
	for k, v := range c.Env {
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

func (c Container) startContainer(cli *client.Client) (err error) {
	log.Info("starting container")
	ctx := context.Background()
	cid, err := c.cId(cli)
	if err != nil {
		return err
	}
	return cli.ContainerStart(ctx, cid, container.StartOptions{})
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
	err = c.Clear()
	if err != nil {
		return err
	}
	firewall, err := c.firewall(make([]Port, 0))
	if err != nil {
		return err
	}
	err = firewall.Uninstall()
	if err != nil {
		return err
	}
	return c.deleteUser()
}

func (c Container) Clear() error {
	log.Info("clearing data")
	return exec.Command("rm", "-rf", filepath.Join(c.Dir(), "**")).Run()
}
