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

package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// NodeInternalNetworkCheck makes sure the node has an address inside the networks the
// cluster was told its nodes talk to each other over.
//
// When it has none, bashible discovers it much later and says "Discovered NodeIP is empty. Check
// out your StaticClusterConfiguration internalNetworkCIDRs" — after the tunnels are up and
// 01-bootstrap-prerequisites has run, and without printing a single address of the node, because
// the diagnostics in bb_node_ip.sh run under `|| true` and go to a stream nobody sees.
//
// This one prints the addresses.
type NodeInternalNetworkCheck struct {
	MetaConfig    *config.MetaConfig
	NodeInterface NodeInterfaceFunc
}

const NodeInternalNetworkCheckName preflight.CheckName = "node-internal-network"

func (NodeInternalNetworkCheck) Description() string {
	return "the node has an address inside StaticClusterConfiguration.internalNetworkCIDRs"
}

func (NodeInternalNetworkCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeInternalNetworkCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeInternalNetworkCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("the cluster configuration was not loaded")
	}

	declared, err := declaredInternalNetworkCIDRs(c.MetaConfig)
	if err != nil {
		return "", err
	}
	if len(declared) == 0 {
		// With no internalNetworkCIDRs the node picks its address itself, which is what a
		// single-interface node wants.
		return "", preflight.NotApplicable("StaticClusterConfiguration declares no internalNetworkCIDRs")
	}

	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	state, err := collectHostNetworkState(ctx, nodeInterface)
	if err != nil {
		return "", scriptFailure("read the network configuration", nodeInterface, nil, err)
	}

	addresses := nodeAddresses(state)
	if len(addresses) == 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the addresses of %s", host),
			Observed: "the node reported none",
			Expected: "at least one address on an interface",
			Fix:      "configure a network interface on the node",
		})
	}

	for _, declaredCIDR := range declared {
		prefix, err := netip.ParsePrefix(declaredCIDR)
		if err != nil {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  "StaticClusterConfiguration.internalNetworkCIDRs",
				Observed: fmt.Sprintf("%q is not a CIDR", declaredCIDR),
				Expected: "an address and a prefix length, for example 192.168.1.0/24",
				Fix:      "correct StaticClusterConfiguration.internalNetworkCIDRs",
			})
		}

		for _, address := range addresses {
			parsed, err := netip.ParseAddr(address)
			if err != nil {
				continue
			}
			if prefix.Masked().Contains(parsed) {
				return fmt.Sprintf("%s has %s, inside StaticClusterConfiguration.internalNetworkCIDRs entry %s", host, address, declaredCIDR), nil
			}
		}
	}

	return "", preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("the addresses of %s against StaticClusterConfiguration.internalNetworkCIDRs", host),
		Observed: fmt.Sprintf("the node has %s, and none of them is inside %s", strings.Join(addresses, ", "), strings.Join(declared, ", ")),
		Expected: "an address of the node inside one of the declared networks",
		Fix: "add the network the node is actually on to StaticClusterConfiguration.internalNetworkCIDRs, " +
			"or give the node an address inside one of them",
	})
}

func declaredInternalNetworkCIDRs(meta *config.MetaConfig) ([]string, error) {
	raw, ok := meta.StaticClusterConfig["internalNetworkCIDRs"]
	if !ok || len(raw) == 0 {
		return nil, nil
	}

	var cidrs []string
	if err := json.Unmarshal(raw, &cidrs); err != nil {
		return nil, fmt.Errorf("StaticClusterConfiguration.internalNetworkCIDRs is not a list of CIDRs")
	}
	return cidrs, nil
}

// nodeAddresses is every address the node reported on an interface, deduplicated and ordered so
// the message reads the same on every run.
//
// The addresses, not the networks they are on: the operator needs to see 192.168.1.15, which is
// what the node has, rather than 192.168.1.0, which is where it lives.
func nodeAddresses(state hostNetworkState) []string {
	seen := map[string]struct{}{}
	for _, address := range state.Addresses {
		if !strings.HasPrefix(address.Source, "interface ") {
			continue
		}
		seen[address.Address] = struct{}{}
	}

	addresses := make([]string, 0, len(seen))
	for address := range seen {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	return addresses
}

func NodeInternalNetwork(metaConfig *config.MetaConfig, nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeInternalNetworkCheck{MetaConfig: metaConfig, NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeInternalNetworkCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
