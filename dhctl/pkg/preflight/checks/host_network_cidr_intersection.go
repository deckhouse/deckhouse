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
	MetaConfig    *config.MetaConfig
	NodeInterface libcon.Interface
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
	return preflight.DefaultRetryPolicy
}

func (c HostNetworkCIDRIntersectionCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return fmt.Errorf("metaConfig is required")
	}
	if c.NodeInterface == nil {
		return fmt.Errorf("node interface is required")
	}

	podCIDR, serviceCIDR, err := getCIDRs(c.MetaConfig)
	if err != nil {
		return err
	}

	host, err := collectHostNetworkState(ctx, c.NodeInterface)
	if err != nil {
		return err
	}

	return checkClusterCIDRsAgainstHost(
		podCIDR,
		serviceCIDR,
		host,
	)
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

	networks, err := parseIPAddresses(addressOutput)
	if err != nil {
		return hostNetworkState{}, err
	}
	state.Networks = append(state.Networks, networks...)

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

func checkClusterCIDRsAgainstHost(
	podCIDR string,
	serviceCIDR string,
	host hostNetworkState,
) error {
	clusterNetworks := []struct {
		name string
		cidr string
	}{
		{
			name: "podSubnetCIDR",
			cidr: podCIDR,
		},
		{
			name: "serviceSubnetCIDR",
			cidr: serviceCIDR,
		},
	}

	for _, clusterNetwork := range clusterNetworks {
		clusterPrefix, err := netip.ParsePrefix(clusterNetwork.cidr)
		if err != nil {
			return fmt.Errorf(
				"invalid %s %q: %w",
				clusterNetwork.name,
				clusterNetwork.cidr,
				err,
			)
		}
		clusterPrefix = clusterPrefix.Masked()

		for _, detected := range host.Networks {
			detectedPrefix, err := netip.ParsePrefix(detected.CIDR)
			if err != nil {
				return fmt.Errorf(
					"invalid CIDR %q discovered from %s: %w",
					detected.CIDR,
					detected.Source,
					err,
				)
			}

			if clusterPrefix.Overlaps(detectedPrefix.Masked()) {
				return fmt.Errorf(
					"%s %s intersects with %s %s",
					clusterNetwork.name,
					clusterNetwork.cidr,
					detected.Source,
					detected.CIDR,
				)
			}
		}

		for _, detected := range host.Addresses {
			detectedAddress, err := netip.ParseAddr(detected.Address)
			if err != nil {
				return fmt.Errorf(
					"invalid IP address %q discovered from %s: %w",
					detected.Address,
					detected.Source,
					err,
				)
			}

			if clusterPrefix.Contains(detectedAddress) {
				return fmt.Errorf(
					"%s %s contains %s %s",
					clusterNetwork.name,
					clusterNetwork.cidr,
					detected.Source,
					detected.Address,
				)
			}
		}
	}

	return nil
}

func parseIPAddresses(output []byte) ([]detectedNetwork, error) {
	var entries []ipAddressEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("parse ip address output: %w", err)
	}

	var networks []detectedNetwork

	for _, entry := range entries {
		for _, addressInfo := range entry.Addresses {
			if addressInfo.Family != "inet" &&
				addressInfo.Family != "inet6" {
				continue
			}

			address, err := netip.ParseAddr(addressInfo.LocalAddress)
			if err != nil {
				return nil, fmt.Errorf(
					"parse address %q on interface %s: %w",
					addressInfo.LocalAddress,
					entry.InterfaceName,
					err,
				)
			}

			if addressInfo.PrefixLength < 0 ||
				addressInfo.PrefixLength > address.BitLen() {
				return nil, fmt.Errorf(
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

			networks = append(networks, detectedNetwork{
				CIDR:   prefix.String(),
				Source: "interface " + entry.InterfaceName,
			})
		}
	}

	return networks, nil
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
	nodeInterface libcon.Interface,
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
		Run:         check.Run,
	}
}
