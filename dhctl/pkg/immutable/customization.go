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
	"cmp"
	"context"
	"fmt"
	"net"
	"slices"

	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// Customization is the machine-specific part of a node's configuration: what
// the cluster cannot know. The set matches what node-controller keeps once the
// node registers (keepBootstrapOnlyFields, internal/controller/nodeconfig/render.go).
type Customization struct {
	// NodeName is the node this document is about, matched against the hosts
	// named on the command line.
	NodeName string

	network *network
	storage *storage
	nodeIP  string
}

// AddressAfterInstall is where the machine answers once it has installed
// itself: the push address, unless its document moves it to a static one.
// Everything after the push — the handoff channel and the apiserver — goes there.
func (c *Customization) AddressAfterInstall(pushAddress string) string {
	if c == nil {
		return pushAddress
	}
	// The document's own nodeIP wins: the machine check has confirmed it is one of
	// the addresses this document gives it, and on a machine with several NICs it
	// is not necessarily the first one.
	if c.nodeIP != "" {
		return c.nodeIP
	}
	if c.network == nil {
		return pushAddress
	}

	for _, iface := range c.network.Interfaces {
		if iface.DHCP || len(iface.Addresses) == 0 {
			continue
		}
		host, _, err := net.ParseCIDR(iface.Addresses[0])
		if err != nil {
			// ParseCustomizations refuses an address without a prefix length, so this
			// is unreachable from a document — and the push address is where the
			// machine answers now, which beats killing the installer mid-run.
			return pushAddress
		}
		return host.String()
	}

	return pushAddress
}

// customizationDocument is the shape the operator writes. Deliberately not the
// full NodeConfig: a field outside this set is either a typo or a NodeGroup
// setting in the wrong place, and both are worth refusing.
type customizationDocument struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   objectMeta `json:"metadata"`
	Spec       struct {
		Network *network `json:"network,omitempty"`
		Storage *storage `json:"storage,omitempty"`
		Kubelet struct {
			NodeIP string `json:"nodeIP,omitempty"`
		} `json:"kubelet,omitempty"`
	} `json:"spec"`
}

// ParseCustomizations reads the NodeConfig documents the operator put next to
// ClusterConfiguration. Strict on purpose: a silently dropped field is a
// machine that boots with a configuration nobody asked for.
func ParseCustomizations(ctx context.Context, documents []string) ([]Customization, error) {
	customizations := make([]Customization, 0, len(documents))

	for i, document := range documents {
		var parsed customizationDocument
		if err := yaml.UnmarshalStrict([]byte(document), &parsed); err != nil {
			return nil, fmt.Errorf(
				"read node customization %d: %w. A machine is described by network, storage and kubelet.nodeIP only; dhctl puts no other field of this document in the payload",
				i+1, err)
		}
		if parsed.APIVersion != PayloadAPIVersion || parsed.Kind != NodeConfigKind {
			return nil, fmt.Errorf("node customization %d is %s/%s, expected %s/%s",
				i+1, parsed.APIVersion, parsed.Kind, PayloadAPIVersion, NodeConfigKind)
		}
		if parsed.Metadata.Name == "" {
			return nil, fmt.Errorf("node customization %d names no node in metadata.name", i+1)
		}
		if parsed.Spec.Network != nil && parsed.Spec.Network.Hostname != "" {
			return nil, fmt.Errorf(
				"node customization %d sets network.hostname; the hostname is the node name, which the cluster decides", i+1)
		}
		var selector *diskSelector
		var mounts []mount
		if parsed.Spec.Storage != nil {
			selector = parsed.Spec.Storage.DiskSelector
			mounts = parsed.Spec.Storage.Mounts
		}
		// An empty selector matches every disk, config drives included, where the
		// render had one narrowed by size.
		if selector != nil && *selector == (diskSelector{}) {
			return nil, fmt.Errorf(
				"node customization %d has an empty storage.diskSelector; name at least one attribute of the disk or leave the block out", i+1)
		}
		if err := refuseRenderedMountOverrides(mounts, i+1); err != nil {
			return nil, err
		}
		if err := checkAddresses(ctx, parsed.Spec.Network, parsed.Metadata.Name); err != nil {
			return nil, err
		}

		customizations = append(customizations, Customization{
			NodeName: parsed.Metadata.Name,
			network:  parsed.Spec.Network,
			storage:  parsed.Spec.Storage,
			nodeIP:   parsed.Spec.Kubelet.NodeIP,
		})
	}

	return customizations, nil
}

// refuseRenderedMountOverrides refuses bindTo and mode on a mount that
// overrides a rendered one: any bindTo but the rendered one moves etcd off the
// hostPath its static pod carries, so there is no value worth accepting.
func refuseRenderedMountOverrides(mounts []mount, number int) error {
	rendered := etcdMounts()

	for _, m := range mounts {
		if !slices.ContainsFunc(rendered, func(r mount) bool { return r.Name == m.Name }) {
			continue
		}
		if m.BindTo == "" && m.Mode == "" {
			continue
		}
		return fmt.Errorf(
			"node customization %d sets bindTo or mode on the %s mount, which the installer renders itself: leave both out and name the disk with partitionSelector or device",
			number, m.Name)
	}

	return nil
}

// checkAddresses draws the line the node's renderer draws: refuse what the
// machine cannot honour, warn about what it will ignore. Address= is emitted in
// the static branch only (renderNetwork, images/init/src/0.1/generate.go).
func checkAddresses(ctx context.Context, n *network, nodeName string) error {
	if n == nil {
		return nil
	}

	for _, iface := range n.Interfaces {
		// A DHCP interface never has its addresses read, so they are a mistake we
		// survive: most likely an operator who meant dhcp: false.
		if iface.DHCP {
			if len(iface.Addresses) > 0 {
				dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
					"Node %s gives interface %s the addresses %v while leaving it on dhcp: true; the machine takes its address over DHCP and ignores them. Write dhcp: false to have them configured",
					nodeName, iface.Name, iface.Addresses))
			}
			continue
		}
		// The interfaces replace the rendered ones wholesale, and the machine emits
		// neither Address= nor DHCP= for this one: the NIC comes up unconfigured and
		// the node never registers. The CRD defaults dhcp to false, so this is easy.
		if len(iface.Addresses) == 0 {
			return fmt.Errorf(
				"node %s leaves interface %s on dhcp: false and gives it no address, so the machine configures it with neither and never joins: "+
					"give the interface an address, or write dhcp: true",
				nodeName, iface.Name)
		}
		// A missing prefix means a single host to networkd, which leaves the
		// machine off its own subnet with no way to say so.
		for _, address := range iface.Addresses {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf(
					"node %s gives interface %s the address %q, which carries no prefix length: write it as 192.168.0.101/24, the form the machine configures the interface with",
					nodeName, iface.Name, address)
			}
		}
	}

	return nil
}

// applyCustomization overlays what the operator said about this machine onto
// the rendered spec, field by field: a document names the one thing the cluster
// got wrong, and what it leaves out must keep what the render put there.
func applyCustomization(spec *nodeSpec, c Customization) {
	if c.network != nil {
		// The hostname is never taken from the document: it has to match the node
		// name however the machine was given its addresses.
		if len(c.network.Interfaces) > 0 {
			spec.Network.Interfaces = c.network.Interfaces
		}
		if c.network.DNS != nil {
			spec.Network.DNS = c.network.DNS
		}
		if len(c.network.Routes) > 0 {
			spec.Network.Routes = c.network.Routes
		}
	}
	if c.storage != nil {
		// Selector and device move together: the CRD gives diskSelector priority
		// over device, so a document naming only device has to clear the selector.
		if c.storage.DiskSelector != nil || c.storage.Device != "" {
			spec.Storage.DiskSelector = c.storage.DiskSelector
			spec.Storage.Device = c.storage.Device
		}
		spec.Storage.Mounts = mergeMounts(spec.Storage.Mounts, c.storage.Mounts)
	}
	if c.nodeIP != "" {
		spec.Kubelet.NodeIP = c.nodeIP
	}
}

// defaultNodeIP names the address kubelet registers the node under when nobody
// else did. Left unset, the node picks the interface carrying the default route,
// which on a machine with several networks is not necessarily the one the
// cluster reaches it on — and the cluster PKI is issued for this address.
func defaultNodeIP(spec *nodeSpec, address string) {
	if spec.Kubelet.NodeIP == "" {
		spec.Kubelet.NodeIP = address
	}
}

// mergeMounts overlays the operator's mounts onto the rendered ones by name —
// the CRD's own list key. Replacing the slice would drop the etcd disk from a
// control-plane node the moment an operator adds a mount of their own.
func mergeMounts(rendered, custom []mount) []mount {
	merged := slices.Clone(rendered)

	for _, m := range custom {
		i := slices.IndexFunc(merged, func(r mount) bool { return r.Name == m.Name })
		if i < 0 {
			merged = append(merged, m)
			continue
		}
		// The parse refuses both on a mount of a rendered name, so here they can
		// only fall back: etcd stays on the /var/lib/etcd hostPath its static pod
		// carries. Device and PartitionSelector still move wholesale.
		m.BindTo = cmp.Or(m.BindTo, merged[i].BindTo)
		m.Mode = cmp.Or(m.Mode, merged[i].Mode)
		merged[i] = m
	}

	return merged
}
