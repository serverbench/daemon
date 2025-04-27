package hardware

import (
	"github.com/zcalusic/sysinfo"
)

type Memory struct {
	Size  uint   `json:"size"`
	Speed uint   `json:"speed"`
	Type  string `json:"type"`
}

func getMemory(si sysinfo.SysInfo) Memory {
	return Memory{
		Size:  si.Memory.Size,
		Speed: si.Memory.Speed,
		Type:  si.Memory.Type,
	}
}
