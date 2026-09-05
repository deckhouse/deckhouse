// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package immutable

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// NodeAddressAfterInstall is where the machine answers once it has installed
// itself: the address on the interface the cluster reaches it on. Everything
// after the push — the handoff channel and the apiserver — goes there.
//
// The same choice the document is built from, made from the same inputs, so the
// address dhctl dials and the interface the node registers under cannot differ.
func NodeAddressAfterInstall(metaConfig *config.MetaConfig, c *Customization, inventory *Inventory, pushAddress string) (string, error) {
	cidrs, err := clusterInternalNetworkCIDRs(metaConfig)
	if err != nil {
		return "", err
	}

	chosen, err := SelectClusterInterface(ClusterInterfaceInput{
		Inventory:            inventory,
		Customization:        c,
		InternalNetworkCIDRs: cidrs,
		PushAddress:          pushAddress,
	})
	if err != nil {
		return "", err
	}
	return chosen.Address, nil
}

// ClusterInterface is the interface the cluster reaches a node on, and the
// address the node answers at once it has installed itself.
type ClusterInterface struct {
	// Name is empty where nothing is known about the machine yet: a cloud machine
	// that does not exist, or an image too old to serve an inventory. The node
	// then resolves the interface itself, against spec.internalNetworkCIDRs.
	Name string
	// Address is where everything after the push goes — the handoff channel and
	// the apiserver.
	Address string
	// ByPushAddress marks the last rung: nothing named the interface, so the one
	// carrying the address dhctl configures the machine at was taken.
	ByPushAddress bool
}

// ClusterInterfaceInput is everything the choice is made from.
type ClusterInterfaceInput struct {
	// Inventory is what the machine reports about itself, nil where it cannot be
	// asked.
	Inventory *Inventory
	// Customization is what the operator wrote about this machine, nil where
	// nothing was written.
	Customization *Customization
	// InternalNetworkCIDRs are the networks the cluster reaches its nodes on.
	InternalNetworkCIDRs []string
	// PushAddress is where dhctl hands the machine its configuration.
	PushAddress string
}

// SelectClusterInterface picks the interface the cluster reaches this node on,
// while the machine is still untouched. The same ladder the node walks over its
// own NICs, run here over the inventory and the operator's document: a machine
// nothing can be decided for is refused before it is handed anything.
func SelectClusterInterface(in ClusterInterfaceInput) (ClusterInterface, error) {
	interfaces := machineInterfaces(in.Inventory, in.Customization)

	if marked := markedInterfaceName(in.Customization); marked != "" {
		return pickMarkedInterface(interfaces, in.Inventory, marked, in.PushAddress)
	}

	candidates := slices.DeleteFunc(interfaces, func(i machineInterface) bool { return len(i.addresses) == 0 })
	// Nothing is known about the machine's addresses. The node has the cluster
	// networks in its own document and resolves the interface itself.
	if len(candidates) == 0 {
		return ClusterInterface{Address: in.PushAddress}, nil
	}

	if len(in.InternalNetworkCIDRs) > 0 {
		return pickInterfaceByCIDR(candidates, in.InternalNetworkCIDRs)
	}
	if len(candidates) == 1 {
		return chosenInterface(candidates[0]), nil
	}
	// The operator described the interfaces but marked none: the first one they
	// wrote down is the one they thought about first.
	if i := slices.IndexFunc(candidates, func(c machineInterface) bool { return c.described }); i >= 0 {
		return chosenInterface(candidates[i]), nil
	}

	return pickInterfaceByPushAddress(candidates, in.PushAddress)
}

// machineInterface is one NIC as dhctl knows it before anything is installed:
// what the machine reports about it, overlaid with what the document says.
type machineInterface struct {
	name string
	// addresses are the IPv4 addresses the interface will carry: the document's
	// where it configures the interface statically, the machine's otherwise.
	addresses []netip.Addr
	described bool
}

// machineInterfaces lists the NICs, the described ones first: that order is the
// document's, and the ladder reads it.
func machineInterfaces(inventory *Inventory, c *Customization) []machineInterface {
	var interfaces []machineInterface

	for _, described := range describedInterfaces(c) {
		interfaces = append(interfaces, machineInterface{
			name:      described.Name,
			addresses: describedAddresses(described, inventory),
			described: true,
		})
	}
	if inventory == nil {
		return interfaces
	}

	for _, reported := range inventory.Interfaces {
		if slices.ContainsFunc(interfaces, func(i machineInterface) bool { return i.name == reported.Name }) {
			continue
		}
		interfaces = append(interfaces, machineInterface{name: reported.Name, addresses: parseAddresses(reported.Addresses)})
	}

	return interfaces
}

// describedAddresses are the addresses a described interface will carry: its
// own where the document configures it statically, and what the machine reports
// where it takes them over DHCP.
func describedAddresses(iface networkInterface, inventory *Inventory) []netip.Addr {
	if !iface.DHCP && len(iface.Addresses) > 0 {
		return parseAddresses(iface.Addresses)
	}
	if inventory == nil {
		return nil
	}
	for _, reported := range inventory.Interfaces {
		if reported.Name == iface.Name {
			return parseAddresses(reported.Addresses)
		}
	}
	return nil
}

// pickMarkedInterface takes the interface the operator marked, and refuses one
// the machine does not have: the mark is the only thing dhctl cannot overrule,
// so a typo in it is a node the cluster never reaches.
func pickMarkedInterface(interfaces []machineInterface, inventory *Inventory, marked, pushAddress string) (ClusterInterface, error) {
	if inventory != nil {
		reported := slices.ContainsFunc(inventory.Interfaces, func(i InventoryInterface) bool { return i.Name == marked })
		if !reported {
			return ClusterInterface{}, fmt.Errorf(
				"the document marks %s as the cluster interface and the machine has no such interface, only: %s",
				marked, describeInterfaces(inventory.Interfaces))
		}
	}

	i := slices.IndexFunc(interfaces, func(c machineInterface) bool { return c.name == marked })
	if i < 0 || len(interfaces[i].addresses) == 0 {
		// The machine has the interface but reports no address on it: it takes one
		// over DHCP once it is installed, and until then it answers where it was
		// pushed to.
		return ClusterInterface{Name: marked, Address: pushAddress}, nil
	}
	return chosenInterface(interfaces[i]), nil
}

// pickInterfaceByCIDR takes the first network the cluster names that this
// machine has an address in: internalNetworkCIDRs is a preference list, not a
// set.
func pickInterfaceByCIDR(candidates []machineInterface, cidrs []string) (ClusterInterface, error) {
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return ClusterInterface{}, fmt.Errorf("read internalNetworkCIDRs entry %q: %w", cidr, err)
		}
		for _, candidate := range candidates {
			for _, address := range candidate.addresses {
				if prefix.Contains(address) {
					return ClusterInterface{Name: candidate.name, Address: address.String()}, nil
				}
			}
		}
	}

	return ClusterInterface{}, fmt.Errorf(
		"no address of this machine (%s) falls in any of internalNetworkCIDRs (%s), so the node would register "+
			"under an address the cluster does not reach it on: correct internalNetworkCIDRs, or mark the cluster "+
			"interface with cluster: true in the machine's NodeConfig",
		describeMachineAddresses(candidates), strings.Join(cidrs, ", "))
}

// pickInterfaceByPushAddress is the last rung: the interface that already holds
// the address dhctl configures the machine at.
func pickInterfaceByPushAddress(candidates []machineInterface, pushAddress string) (ClusterInterface, error) {
	if address, ok := parseAddress(pushAddress); ok {
		for _, candidate := range candidates {
			if slices.Contains(candidate.addresses, address) {
				return ClusterInterface{Name: candidate.name, Address: address.String(), ByPushAddress: true}, nil
			}
		}
	}

	return ClusterInterface{}, fmt.Errorf(
		"nothing says which interface of this machine the cluster reaches it on, and %s — the address dhctl hands "+
			"it its configuration at — is on none of them (%s), which is what a machine behind NAT looks like: "+
			"mark the interface with cluster: true in the machine's NodeConfig, or name the cluster networks in "+
			"internalNetworkCIDRs",
		pushAddress, describeMachineAddresses(candidates))
}

func chosenInterface(candidate machineInterface) ClusterInterface {
	return ClusterInterface{Name: candidate.name, Address: candidate.addresses[0].String()}
}

// markClusterInterface writes the choice into the document. An operator who
// described no interface gets one synthesised from the machine's own inventory:
// the rendered eth0 is a guess most hardware does not answer to.
func markClusterInterface(spec *nodeSpec, chosen ClusterInterface, describedByOperator bool) {
	if chosen.Name == "" {
		return
	}
	if !describedByOperator {
		spec.Network.Interfaces = []networkInterface{{Name: chosen.Name, DHCP: true, Cluster: true}}
		return
	}

	i := slices.IndexFunc(spec.Network.Interfaces, func(iface networkInterface) bool { return iface.Name == chosen.Name })
	if i < 0 {
		// The document replaced the rendered interfaces whole, and the cluster
		// interface is not among them: without this entry the NIC the cluster
		// reaches the node on is never configured.
		spec.Network.Interfaces = append(spec.Network.Interfaces,
			networkInterface{Name: chosen.Name, DHCP: true, Cluster: true})
		return
	}
	spec.Network.Interfaces[i].Cluster = true
}

// markedInterfaceName is the interface the operator marked, empty where they
// marked none. ParseCustomizations has already refused a second mark.
func markedInterfaceName(c *Customization) string {
	for _, iface := range describedInterfaces(c) {
		if iface.Cluster {
			return iface.Name
		}
	}
	return ""
}

// describedInterfaces are the interfaces the operator's document names, in the
// order it names them.
func describedInterfaces(c *Customization) []networkInterface {
	if c == nil || c.network == nil {
		return nil
	}
	return c.network.Interfaces
}

// parseAddresses keeps the IPv4 addresses of a list written as CIDRs or bare.
// IPv4 only: internalNetworkCIDRs is IPv4 by contract, and the cluster address
// is what kubelet registers the node under.
func parseAddresses(raw []string) []netip.Addr {
	addresses := make([]netip.Addr, 0, len(raw))
	for _, value := range raw {
		if address, ok := parseAddress(value); ok {
			addresses = append(addresses, address)
		}
	}
	return addresses
}

func parseAddress(value string) (netip.Addr, bool) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr(), prefix.Addr().Is4()
	}
	address, err := netip.ParseAddr(value)
	return address, err == nil && address.Is4()
}

func describeMachineAddresses(interfaces []machineInterface) string {
	described := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses := make([]string, 0, len(iface.addresses))
		for _, address := range iface.addresses {
			addresses = append(addresses, address.String())
		}
		described = append(described, iface.name+" ("+strings.Join(addresses, ", ")+")")
	}
	return strings.Join(described, "; ")
}
