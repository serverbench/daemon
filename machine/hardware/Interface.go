package hardware

import (
	"errors"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/zcalusic/sysinfo"
	gnet "net"
	"os"
	"strings"
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
			if !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() {
				addrs = append(addrs, Address{
					Ip:      addr.Addr,
					Version: version,
				})
			}
		}
		if len(addrs) > 0 {
			interfaces = append(interfaces, Interface{
				Addresses: addrs,
				Name:      iface.Name,
				Speed:     uint64(speedMap[iface.Name]),
			})
		}
	}
	if len(interfaces) == 0 && strings.ToLower(os.Getenv("TEST_ETH0")) == "true" {
		interfaces = append(interfaces, Interface{
			Addresses: []Address{{
				Ip:      "1.1.1.1/32",
				Version: "IPv4",
			}},
			Speed: 1,
			Name:  "eth0",
		})
	}
	if len(interfaces) == 0 {
		return nil, errors.New("no interfaces found. the device must be directly addressed using at least a public (non-private) IP")
	}
	return interfaces, err
}
