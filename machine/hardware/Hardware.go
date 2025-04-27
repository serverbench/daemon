package hardware

import (
	"errors"
	"github.com/docker/docker/client"
	"github.com/zcalusic/sysinfo"
	"os"
	"os/user"
)

type Hardware struct {
	CPUs       []CPU       `json:"cpus"`
	Memory     Memory      `json:"memory"`
	Storage    Storage     `json:"storage"`
	Interfaces []Interface `json:"interfaces"`
	Hostname   string      `json:"hostname"`
}

func GetHardware(cli *client.Client) (hardware *Hardware, err error) {
	current, err := user.Current()
	if err != nil {
		return hardware, err
	}
	if current.Uid != "0" {
		return hardware, errors.New("not root")
	}
	var si sysinfo.SysInfo
	si.GetSysInfo()

	cpus, err := GetCPUs()
	if err != nil {
		return hardware, err

	}
	memory := getMemory(si)
	interfaces, err := GetInterfaces(si)
	if err != nil {
		return hardware, err
	}
	storage, err := GetStorage(cli)
	if err != nil {
		return hardware, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return hardware, err
	}
	hardware = &Hardware{
		CPUs:       cpus,
		Interfaces: interfaces,
		Memory:     memory,
		Storage:    *storage,
		Hostname:   hostname,
	}
	return hardware, err
}
