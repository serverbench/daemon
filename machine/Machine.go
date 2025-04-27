package machine

import (
	"errors"
	"github.com/docker/docker/client"
	"os"
	"supervisor/machine/hardware"
)

type Machine struct {
	Id       string            `json:"id"`
	Hardware hardware.Hardware `json:"hardware"`
	Key      string            `json:"key"`
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
	return &Machine{
		Key:      key,
		Hardware: *hw,
	}, nil
}
