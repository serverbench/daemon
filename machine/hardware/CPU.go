package hardware

import (
	"github.com/shirou/gopsutil/v3/cpu"
)

type CPU struct {
	Model     string  `json:"model"`
	Vendor    string  `json:"vendor"`
	Frequency float64 `json:"frequency"`
	Path      string  `json:"path"`
	Cores     int     `json:"cores"`
}

func GetCPUs() ([]CPU, error) {
	infos, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	var cpus []CPU

	for _, info := range infos {
		cpus = append(cpus, CPU{
			Model:     info.ModelName,
			Vendor:    info.VendorID,
			Frequency: info.Mhz,
			Path:      info.CoreID,
		})
	}

	return cpus, nil
}
