package hardware

import (
	"github.com/shirou/gopsutil/v4/net"
	"github.com/zcalusic/sysinfo"
	gnet "net"
)

type Address struct {
	Ip      string `json:"ip"`
	Version string `json:"version"`
}

type Interface struct {
	Speed     uint64    `json:"speed"`
	Name      string    `json:"name"`
	Addresses []Address `json:"addresses"` // will only show public ips
}

func GetInterfaces(si sysinfo.SysInfo) (interfaces []Interface, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	interfaces = make([]Interface, 0)
	speedMap := make(map[string]uint)
	for _, iface := range si.Network {
		speedMap[iface.Name] = iface.Speed
	}
	for _, iface := range ifaces {
		if iface.Name == "docker0" || iface.Name == "lo" {
			continue
		}
		var addrs = make([]Address, 0)
		for _, addr := range iface.Addrs {
			ip, _, err := gnet.ParseCIDR(addr.Addr)
			if err != nil {
				return nil, err
			}
			if ip.IsLoopback() {
				continue
			}
			var version string
			if ip.To4() != nil {
				version = "IPv4"
			} else {
				version = "IPv6"
			}
			addrs = append(addrs, Address{
				Ip:      addr.Addr,
				Version: version,
			})
		}
		interfaces = append(interfaces, Interface{
			Addresses: addrs,
			Name:      iface.Name,
			Speed:     uint64(speedMap[iface.Name]),
		})
	}
	return interfaces, err
}
