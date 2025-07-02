package containers

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	log "github.com/sirupsen/logrus"
	"net"
	"os/exec"
	"strconv"
)

const tcp = "tcp"
const udp = "udp"

var protocols = []string{tcp, udp}

const (
	table     = "filter"
	forward   = "serverbench"
	tlForward = "DOCKER-USER"
	hostNetNS = "/mnt/host_netns"
)

func nsenterIptables(args ...string) error {
	cmdArgs := append([]string{"--net=" + hostNetNS, "iptables"}, args...)
	cmd := exec.Command("nsenter", cmdArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables %v failed: %v: %s", args, err, stderr.String())
	}
	return nil
}

type Firewall struct {
	Chain    string
	Address  string
	Iptables *iptables.IPTables
	Ports    []Port
}

func (c *Container) firewall(ports []Port) (firewall Firewall, err error) {
	ip := net.ParseIP(c.Address)
	if ip == nil {
		return firewall, errors.New("invalid address")
	}
	var path string
	if ip.To4() == nil {
		path = "/wrapper/ip6tables"
	} else {
		path = "/wrapper/iptables"
	}
	instance, err := iptables.New(iptables.Path(path))
	if err != nil {
		return firewall, err
	}
	log.Info("firewall created for ", c.Address)
	firewall = Firewall{
		Chain:    "sb-" + c.Id,
		Address:  c.Address,
		Iptables: instance,
		Ports:    ports,
	}
	return firewall, err
}

// Install refreshes or installs firewall
func (f Firewall) Install() (err error) {
	err = f.ensureParentChain()
	if err != nil {
		return err
	}
	log.Info("installing chain")
	err = f.ensureChain()
	if err != nil {
		return err
	}
	err = f.flushChain()
	if err != nil {
		return err
	}
	for _, port := range f.Ports {
		err = f.securePort(port)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f Firewall) Uninstall() (err error) {
	log.Info("uninstalling chain")
	err = f.ensureChain()
	if err != nil {
		return err
	}
	return f.deleteChain()
}

func (f Firewall) deleteChain() error {
	log.Info("deleting chain rules")

	// Remove the jump rule from FORWARD
	err := f.Iptables.DeleteIfExists(table, forward, "-j", f.Chain)
	if err != nil {
		return fmt.Errorf("failed to remove jump rule from FORWARD: %w", err)
	}

	// Delete the custom chain
	err = f.Iptables.ClearAndDeleteChain(table, f.Chain)
	if err != nil {
		return fmt.Errorf("failed to delete chain: %w", err)
	}
	return nil
}

// flush chain rules
func (f Firewall) flushChain() (err error) {
	log.Info("flushing chain")
	return f.Iptables.ClearChain(table, f.Chain)
}

// creates the rules for securing that port
func (f Firewall) securePort(port Port) (err error) {
	log.Info("securing port")
	var unmatchPolicy string
	if port.Policy == Drop {
		unmatchPolicy = Accept
	} else {
		unmatchPolicy = Drop
	}
	for _, remote := range port.Remotes {
		for _, protocol := range protocols {
			err = f.Iptables.AppendUnique(table, f.Chain, "-p", protocol, "-m", "conntrack", "--ctorigsrc", remote, "--ctorigdst", f.Address, "--ctorigdstport", strconv.Itoa(port.Port), "-j", unmatchPolicy)
			if err != nil {
				return err
			}
		}
	}
	for _, protocol := range protocols {
		err = f.Iptables.AppendUnique(table, f.Chain, "-p", protocol, "-m", "conntrack", "--ctorigdst", f.Address, "--ctorigdstport", strconv.Itoa(port.Port), "-j", port.Policy)
		if err != nil {
			return err
		}
	}
	log.Info("secured port")
	return err
}

func (f Firewall) ensureParentChain() (err error) {
	log.Info("ensuring parent chain")
	exists, err := f.Iptables.ChainExists(table, forward)
	if err != nil {
		return err
	}
	if !exists {
		log.Info("parent chain was missing, creating chain")
		err = f.Iptables.NewChain(table, forward)
		log.Info("created parent chain, adding default forwarding rule")
		err = f.Iptables.InsertUnique(table, tlForward, 1, "-j", forward)
		if err != nil {
			return err
		}
		log.Info("parent chain setup finished")
	}
	return err
}

// creates the priority chain to bypass docker default iptables
func (f Firewall) ensureChain() (err error) {
	log.Info("retrieving chain")
	exists, err := f.Iptables.ChainExists(table, f.Chain)
	if err != nil {
		return err
	}
	if !exists {
		log.Info("chain was missing, creating chain")
		err = f.Iptables.NewChain(table, f.Chain)
		if err != nil {
			return err
		}
		log.Info("created chain, adding default forwarding rule")
		err = f.Iptables.AppendUnique(table, forward, "-j", f.Chain)
		if err != nil {
			return err
		}
		log.Info("chain setup finished")
	}
	return nil
}
