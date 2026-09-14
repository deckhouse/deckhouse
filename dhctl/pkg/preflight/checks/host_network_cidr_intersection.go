// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type HostNetworkCIDRIntersectionCheck struct {
	MetaConfig *config.MetaConfig
	// NodeInterface is resolved when the check runs, not when its suite is built: see
	// NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
}

const HostNetworkCIDRIntersectionCheckName preflight.CheckName = "host-network-cidr-intersection"

type detectedNetwork struct {
	CIDR   string
	Source string
}

type detectedAddress struct {
	Address string
	Source  string
}

type hostNetworkState struct {
	Networks  []detectedNetwork
	Addresses []detectedAddress
}

type ipAddressEntry struct {
	InterfaceName string          `json:"ifname"`
	Addresses     []ipAddressInfo `json:"addr_info"`
}

type ipAddressInfo struct {
	Family       string `json:"family"`
	LocalAddress string `json:"local"`
	PrefixLength int    `json:"prefixlen"`
}

type ipRouteEntry struct {
	Destination string `json:"dst"`
	Gateway     string `json:"gateway"`
	Device      string `json:"dev"`
}

func (HostNetworkCIDRIntersectionCheck) Description() string {
	return "cluster CIDRs do not intersect with host networks"
}

func (HostNetworkCIDRIntersectionCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (HostNetworkCIDRIntersectionCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c HostNetworkCIDRIntersectionCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("metaConfig is required")
	}
	if c.NodeInterface == nil {
		return "", fmt.Errorf("node interface is required")
	}

	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}

	podCIDR, serviceCIDR, err := getCIDRs(c.MetaConfig)
	if err != nil {
		return "", err
	}

	host, err := collectHostNetworkState(ctx, nodeInterface)
	if err != nil {
		return "", scriptFailure("read the network configuration", nodeInterface, nil, err)
	}

	conflict, err := findClusterCIDRConflict(podCIDR, serviceCIDR, host)
	if err != nil {
		return "", err
	}
	if conflict != nil {
		// A network the node is already on is not going to move between two attempts.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("ClusterConfiguration.%s against the networks of %s", conflict.field, hostPhrase(nodeInterface)),
			Observed: conflict.observed,
			Expected: "cluster CIDRs that no network or address of the node falls inside",
			Fix: fmt.Sprintf("change ClusterConfiguration.%s to a range the node does not use; "+
				"it cannot be changed after the cluster is created", conflict.field),
		})
	}

	return fmt.Sprintf("podSubnetCIDR %s and serviceSubnetCIDR %s do not overlap the networks of %s",
		podCIDR, serviceCIDR, hostPhrase(nodeInterface)), nil
}

// cidrConflict is one overlap, named in the reader's vocabulary on both sides.
type cidrConflict struct {
	field    string
	observed string
}

func collectHostNetworkState(
	ctx context.Context,
	nodeInterface libcon.Interface,
) (hostNetworkState, error) {
	var state hostNetworkState

	addressOutput, err := hostCommandOutput(
		ctx,
		nodeInterface,
		"ip",
		"-j",
		"address",
		"show",
	)
	if err != nil {
		return hostNetworkState{}, err
	}

	addressState, err := parseIPAddresses(addressOutput)
	if err != nil {
		return hostNetworkState{}, err
	}
	state.Networks = append(state.Networks, addressState.Networks...)
	state.Addresses = append(state.Addresses, addressState.Addresses...)

	for _, family := range []string{"-4", "-6"} {
		routeOutput, err := hostCommandOutput(
			ctx,
			nodeInterface,
			"ip",
			"-j",
			family,
			"route",
			"show",
			"table",
			"all",
		)
		if err != nil {
			return hostNetworkState{}, err
		}

		routeState, err := parseIPRoutes(routeOutput)
		if err != nil {
			return hostNetworkState{}, err
		}

		state.Networks = append(
			state.Networks,
			routeState.Networks...,
		)
		state.Addresses = append(
			state.Addresses,
			routeState.Addresses...,
		)
	}

	resolvConfOutput, err := hostCommandOutput(
		ctx,
		nodeInterface,
		"cat",
		"/etc/resolv.conf",
	)
	if err != nil {
		return hostNetworkState{}, err
	}

	nameservers, err := parseResolvConf(resolvConfOutput)
	if err != nil {
		return hostNetworkState{}, err
	}
	state.Addresses = append(state.Addresses, nameservers...)

	return state, nil
}

func hostCommandOutput(
	ctx context.Context,
	nodeInterface libcon.Interface,
	name string,
	args ...string,
) ([]byte, error) {
	command := nodeInterface.Command(name, args...)

	stdout, stderr, err := command.Output(ctx)
	if err == nil {
		return stdout, nil
	}

	stderrMessage := strings.TrimSpace(string(stderr))
	if stderrMessage == "" {
		return nil, fmt.Errorf(
			"execute host command %s: %w",
			name,
			err,
		)
	}

	return nil, fmt.Errorf(
		"execute host command %s: %w: %s",
		name,
		err,
		stderrMessage,
	)
}

// findClusterCIDRConflict looks for the first cluster CIDR that overlaps something the node is
// already using. The returned error is for input that could not be read at all; an overlap comes
// back as a *cidrConflict, so the caller can phrase it with the host it was found on.
func findClusterCIDRConflict(
	podCIDR string,
	serviceCIDR string,
	host hostNetworkState,
) (*cidrConflict, error) {
	clusterNetworks := []struct {
		name string
		cidr string
	}{
		{name: "podSubnetCIDR", cidr: podCIDR},
		{name: "serviceSubnetCIDR", cidr: serviceCIDR},
	}

	for _, clusterNetwork := range clusterNetworks {
		clusterPrefix, err := netip.ParsePrefix(clusterNetwork.cidr)
		if err != nil {
			return nil, preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("ClusterConfiguration.%s", clusterNetwork.name),
				Observed: fmt.Sprintf("%q is not a CIDR: %s", clusterNetwork.cidr, err),
				Expected: "an IPv4 range in CIDR notation",
				Fix:      fmt.Sprintf("write %s as an address and a prefix, for example 10.111.0.0/16", clusterNetwork.name),
			})
		}
		clusterPrefix = clusterPrefix.Masked()

		for _, detected := range host.Networks {
			detectedPrefix, err := netip.ParsePrefix(detected.CIDR)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q discovered from %s: %w", detected.CIDR, detected.Source, err)
			}

			if clusterPrefix.Overlaps(detectedPrefix.Masked()) {
				return &cidrConflict{
					field:    clusterNetwork.name,
					observed: fmt.Sprintf("%s %s overlaps %s, which the node uses (%s)", clusterNetwork.name, clusterNetwork.cidr, detected.CIDR, detected.Source),
				}, nil
			}
		}

		for _, detected := range host.Addresses {
			detectedAddress, err := netip.ParseAddr(detected.Address)
			if err != nil {
				return nil, fmt.Errorf("invalid IP address %q discovered from %s: %w", detected.Address, detected.Source, err)
			}

			if clusterPrefix.Contains(detectedAddress) {
				return &cidrConflict{
					field:    clusterNetwork.name,
					observed: fmt.Sprintf("%s %s contains %s, which the node uses (%s)", clusterNetwork.name, clusterNetwork.cidr, detected.Address, detected.Source),
				}, nil
			}
		}
	}

	return nil, nil
}

// parseIPAddresses reads `ip -j address show` into both the networks the interfaces are on and
// the addresses the node itself holds.
//
// It used to keep only the masked network and throw the address away, which meant nothing knew
// what address the node actually has — not the CIDR comparison here, and not the check that asks
// whether any of them falls inside internalNetworkCIDRs, which could only report the network and
// so printed 192.168.1.0 where the operator needed to see 192.168.1.15.
func parseIPAddresses(output []byte) (hostNetworkState, error) {
	var entries []ipAddressEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return hostNetworkState{}, fmt.Errorf("parse ip address output: %w", err)
	}

	var state hostNetworkState

	for _, entry := range entries {
		for _, addressInfo := range entry.Addresses {
			if addressInfo.Family != "inet" &&
				addressInfo.Family != "inet6" {
				continue
			}

			address, err := netip.ParseAddr(addressInfo.LocalAddress)
			if err != nil {
				return hostNetworkState{}, fmt.Errorf(
					"parse address %q on interface %s: %w",
					addressInfo.LocalAddress,
					entry.InterfaceName,
					err,
				)
			}

			if addressInfo.PrefixLength < 0 ||
				addressInfo.PrefixLength > address.BitLen() {
				return hostNetworkState{}, fmt.Errorf(
					"invalid prefix length %d for address %s on interface %s",
					addressInfo.PrefixLength,
					addressInfo.LocalAddress,
					entry.InterfaceName,
				)
			}

			prefix := netip.PrefixFrom(
				address,
				addressInfo.PrefixLength,
			).Masked()

			source := "interface " + entry.InterfaceName
			state.Networks = append(state.Networks, detectedNetwork{
				CIDR:   prefix.String(),
				Source: source,
			})
			state.Addresses = append(state.Addresses, detectedAddress{
				Address: address.String(),
				Source:  source,
			})
		}
	}

	return state, nil
}

func parseIPRoutes(output []byte) (hostNetworkState, error) {
	var routes []ipRouteEntry
	if err := json.Unmarshal(output, &routes); err != nil {
		return hostNetworkState{}, fmt.Errorf(
			"parse ip route output: %w",
			err,
		)
	}

	var state hostNetworkState

	for _, route := range routes {
		if route.Destination == "default" {
			if err := addDefaultGateway(&state, route); err != nil {
				return hostNetworkState{}, err
			}
			continue
		}

		if route.Destination == "" {
			continue
		}

		prefix, err := parseRouteDestination(route.Destination)
		if err != nil {
			return hostNetworkState{}, err
		}

		// Some ip versions may represent the default route
		// as 0.0.0.0/0 or ::/0.
		if prefix.Bits() == 0 {
			if err := addDefaultGateway(&state, route); err != nil {
				return hostNetworkState{}, err
			}
			continue
		}

		source := "route"
		if route.Device != "" {
			source += " via " + route.Device
		}

		state.Networks = append(state.Networks, detectedNetwork{
			CIDR:   prefix.Masked().String(),
			Source: source,
		})
	}

	return state, nil
}

func addDefaultGateway(
	state *hostNetworkState,
	route ipRouteEntry,
) error {
	if route.Gateway == "" {
		return nil
	}

	gateway, err := netip.ParseAddr(route.Gateway)
	if err != nil {
		return fmt.Errorf(
			"parse default gateway %q: %w",
			route.Gateway,
			err,
		)
	}

	source := "default gateway"
	if route.Device != "" {
		source += " via " + route.Device
	}

	state.Addresses = append(
		state.Addresses,
		detectedAddress{
			Address: gateway.String(),
			Source:  source,
		},
	)

	return nil
}

func parseRouteDestination(destination string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(destination); err == nil {
		return prefix.Masked(), nil
	}

	// Host routes may be returned without /32 or /128.
	address, err := netip.ParseAddr(destination)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf(
			"parse route destination %q: %w",
			destination,
			err,
		)
	}

	return netip.PrefixFrom(address, address.BitLen()), nil
}

func parseResolvConf(output []byte) ([]detectedAddress, error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))

	var addresses []detectedAddress
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())
		if line == "" ||
			strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, ";") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}

		address, err := netip.ParseAddr(fields[1])
		if err != nil {
			return nil, fmt.Errorf(
				"parse nameserver %q on line %d: %w",
				fields[1],
				lineNumber,
				err,
			)
		}

		// A scoped IPv6 address may contain an interface zone,
		// but cluster CIDRs do not have zones.
		if address.Zone() != "" {
			address = address.WithZone("")
		}

		addresses = append(addresses, detectedAddress{
			Address: address.String(),
			Source:  "DNS server",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan resolv.conf: %w", err)
	}

	return addresses, nil
}

func HostNetworkCIDRIntersection(
	metaConfig *config.MetaConfig,
	nodeInterface NodeInterfaceFunc,
) preflight.Check {
	check := HostNetworkCIDRIntersectionCheck{
		MetaConfig:    metaConfig,
		NodeInterface: nodeInterface,
	}

	return preflight.Check{
		Name:        HostNetworkCIDRIntersectionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
