package hardware

import (
	"context"
	"errors"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/v3/disk"
)

const containerPath = "/containers"

type Storage struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func GetHostPath(cli *client.Client) (path *string, err error) {
	// Allow the host path to be provided directly via an environment variable,
	// which avoids the Docker self-lookup and is more reliable in some setups.
	if envPath := os.Getenv("HOST_CONTAINERS_PATH"); envPath != "" {
		return &envPath, nil
	}

	// Fall back to discovering the path by inspecting the self container.
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: true, // include stopped containers so a brief stop doesn't cause failure
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "name",
			Value: "^/serverbench$",
		}),
	})
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, errors.New("unknown self container")
	}
	self := containers[0]
	var hostPath string
	for _, m := range self.Mounts {
		if m.Type == mount.TypeBind && m.Destination == containerPath {
			hostPath = m.Source
			break
		}
	}
	if hostPath == "" {
		return nil, errors.New("unknown self container mount")
	}
	return &hostPath, nil
}

func GetStorage(cli *client.Client) (storage *Storage, err error) {
	usage, err := disk.Usage(containerPath)
	if err != nil {
		return nil, err
	}
	hostPath, err := GetHostPath(cli)
	if err != nil {
		return nil, err
	}
	return &Storage{
		Total: usage.Total,
		Used:  usage.Used,
		Path:  *hostPath,
	}, nil
}
