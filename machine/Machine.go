package machine

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"supervisor/containers"
	"supervisor/machine/hardware"
)

const prefix = "sb-"
const completePrefix = "/" + prefix

type Machine struct {
	Id         string                 `json:"id"`
	Hardware   hardware.Hardware      `json:"hardware"`
	Key        string                 `json:"key"`
	Containers []containers.Container `json:"containers"`
}

func GetMachine(cli *client.Client) (machine *Machine, err error) {
	key := os.Getenv("SERVERBENCH_KEY")
	if key == "" {
		key = os.Getenv("KEY")
	}
	if key == "" {
		return machine, errors.New("serverbench key not found")
	}
	hw, err := hardware.GetHardware(cli)
	if err != nil {
		return machine, err
	}
	dockerContainers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: true,
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "name",
			Value: "^/" + prefix,
		}),
	})
	if err != nil {
		return machine, err
	}
	finalContainers := make([]containers.Container, 0)
	for _, dockerContainer := range dockerContainers {
		var id string
		for _, name := range dockerContainer.Names {
			if strings.HasPrefix(name, completePrefix) {
				id = strings.ReplaceAll(name, completePrefix, "")
			}
		}
		specifics, err := cli.ContainerInspect(context.Background(), dockerContainer.ID)
		if err != nil {
			return machine, err
		}
		var address string
		for _, binding := range specifics.HostConfig.PortBindings {
			for _, port := range binding {
				address = port.HostIP
			}
			break
		}
		var mount string
		for _, mnt := range dockerContainer.Mounts {
			mount = mnt.Destination
		}
		finalContainer := containers.Container{
			Id:      id,
			Image:   dockerContainer.Image,
			Address: address,
			Mount:   mount,
			Envs:    map[string]string{},
			Ports:   []containers.Port{},
		}
		err = finalContainer.MountDir()
		if err != nil {
			return machine, err
		}
		finalContainers = append(finalContainers, finalContainer)
	}
	return &Machine{
		Key:        key,
		Hardware:   *hw,
		Containers: finalContainers,
	}, nil
}

func (m *Machine) UpdateContainers(cli *client.Client, newContainers []containers.Container) (err error) {
	toBeCreated := make([]containers.Container, 0)
	toBeDeleted := make(map[string]containers.Container)
	for _, c := range m.Containers {
		toBeDeleted[c.Id] = c
	}
	for _, provided := range newContainers {
		if _, exists := toBeDeleted[provided.Id]; exists {
			delete(toBeDeleted, provided.Id)
		} else {
			toBeCreated = append(toBeCreated, provided)
		}
	}
	for _, deletedContainer := range toBeDeleted {
		err = deletedContainer.Destroy(cli)
		if err != nil {
			return err
		}
	}
	for _, createdContainer := range toBeCreated {
		err = createdContainer.Create(cli)
		if err != nil {
			return err
		}
	}
	m.Containers = newContainers
	log.Info(len(toBeCreated), " created containers, ", len(toBeDeleted), " deleted containers, ", len(newContainers), " final containers")
	return nil
}
