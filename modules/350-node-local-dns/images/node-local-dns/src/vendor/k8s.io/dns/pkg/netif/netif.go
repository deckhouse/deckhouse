package netif

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"net"
)

type NetifManager struct {
	netlink.Handle
	*netlink.Addr
}

// NewNetifManager returns a new instance of NetifManager with the ip address set to the provided value
// This ip address will be bound to any devices created by this instance.
func NewNetifManager(ip net.IP) *NetifManager {
	return &NetifManager{netlink.Handle{}, &netlink.Addr{IPNet: netlink.NewIPNet(ip)}}
}

// EnsureDummyDevice checks for the presence of the given dummy device and creates one if it does not exist.
// Returns a boolean to indicate if this device was found and error if any.
func (m *NetifManager) EnsureDummyDevice(name string) (bool, error) {
	l, err := m.LinkByName(name)
	if err == nil {
		// found dummy device, make sure ip matches. AddrAdd will return error if address exists, will add it otherwise
		m.AddrAdd(l, m.Addr)
		return true, nil
	}
	return false, m.AddDummyDevice(name)
}

// AddDummyDevice creates a dummy device with the given name. It also binds the ip address of the NetifManager instance
// to this device. This function returns an error if the device exists or if address binding fails.
func (m *NetifManager) AddDummyDevice(name string) error {
	_, err := m.LinkByName(name)
	if err == nil {
		return fmt.Errorf("Link %s exists", name)
	}
	dummy := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{Name: name},
	}
	err = m.LinkAdd(dummy)
	if err != nil {
		return err
	}
	l, _ := m.LinkByName(name)
	return m.AddrAdd(l, m.Addr)
}

// RemoveDummyDevice deletes the dummy device with the given name.
func (m *NetifManager) RemoveDummyDevice(name string) error {
	link, err := m.LinkByName(name)
	if err != nil {
		return err
	}
	return m.LinkDel(link)
}
