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
	"net"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// CloudNodeNetworkCIDRIntersectionCheck compares the cluster's pod and service subnets against the
// network the cloud provider will place the nodes on.
//
// Nothing checked this before. The static suite has host-network-cidr-intersection, but it runs
// against a node that already exists; on a cloud cluster the node network is declared in the
// provider configuration, and an overlap there is only discovered after terraform has run for
// fifteen minutes — as "Waiting for Deckhouse to become Ready" timing out, with nothing pointing
// at the subnets.
type CloudNodeNetworkCIDRIntersectionCheck struct {
	MetaConfig *config.MetaConfig
}

const CloudNodeNetworkCIDRIntersectionCheckName preflight.CheckName = "cloud-node-network-cidr-intersection"

func (CloudNodeNetworkCIDRIntersectionCheck) Description() string {
	return "cluster CIDRs do not intersect the cloud network the nodes are placed on"
}

func (CloudNodeNetworkCIDRIntersectionCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (CloudNodeNetworkCIDRIntersectionCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

// nodeNetworkCIDRFields are the names under which the providers declare the network their nodes
// are given addresses on. They are collected by name rather than by a per-provider path because
// each provider offers several layouts, and the field moves between them.
var nodeNetworkCIDRFields = map[string]struct{}{
	"vpcNetworkCIDR":      {}, // aws
	"nodeNetworkCIDR":     {}, // aws, yandex, dynamix
	"vNetCIDR":            {}, // azure
	"subnetCIDR":          {}, // azure
	"subnetworkCIDR":      {}, // gcp
	"internalNetworkCIDR": {}, // openstack, huaweicloud, vcd
	"existingNetworkCIDR": {},
}

func (c CloudNodeNetworkCIDRIntersectionCheck) Run(_ context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("no configuration was loaded from --config")
	}
	if !c.MetaConfig.HasClusterConfiguration() {
		return "", preflight.NotApplicable("there is no ClusterConfiguration to read the subnets from")
	}
	if len(c.MetaConfig.ProviderClusterConfig) == 0 {
		return "", preflight.NotApplicable("there is no %s to read the node network from", providerDocumentKind(c.MetaConfig.ProviderName))
	}

	podCIDR, serviceCIDR, err := getCIDRs(c.MetaConfig)
	if err != nil {
		return "", err
	}

	networks := collectNodeNetworkCIDRs(c.MetaConfig.ProviderClusterConfig, providerDocumentKind(c.MetaConfig.ProviderName))
	if len(networks) == 0 {
		return "", preflight.NotApplicable("%s declares no node network CIDR", providerDocumentKind(c.MetaConfig.ProviderName))
	}

	clusterNetworks := []struct{ name, cidr string }{
		{"podSubnetCIDR", podCIDR},
		{"serviceSubnetCIDR", serviceCIDR},
	}

	// Every overlap, so a configuration that clashes on both subnets is fixed in one pass.
	var overlaps []string
	for _, cluster := range clusterNetworks {
		_, clusterNet, err := net.ParseCIDR(cluster.cidr)
		if err != nil {
			return "", invalidCIDRFailure(cluster.name, cluster.cidr)
		}

		for _, network := range networks {
			_, providerNet, err := net.ParseCIDR(network.cidr)
			if err != nil {
				return "", preflight.Permanent(&preflight.Failure{
					Checked:  network.field,
					Observed: fmt.Sprintf("%q is not a CIDR", network.cidr),
					Expected: "an address and a prefix length, for example 10.241.32.0/24",
					Fix:      fmt.Sprintf("correct %s", network.field),
				})
			}

			if clusterNet.Contains(providerNet.IP) || providerNet.Contains(clusterNet.IP) {
				overlaps = append(overlaps, fmt.Sprintf("%s %s overlaps %s %s",
					cluster.name, cluster.cidr, network.field, network.cidr))
			}
		}
	}

	if len(overlaps) > 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  "the cluster subnets against the cloud network the nodes are created on",
			Observed: "- " + strings.Join(overlaps, "\n- "),
			Expected: "ranges that do not overlap",
			Fix: fmt.Sprintf("change ClusterConfiguration.podSubnetCIDR or ClusterConfiguration.serviceSubnetCIDR, "+
				"or the node network in %s", providerDocumentKind(c.MetaConfig.ProviderName)),
		})
	}

	return fmt.Sprintf("podSubnetCIDR %s and serviceSubnetCIDR %s do not overlap %s",
		podCIDR, serviceCIDR, describeNetworks(networks)), nil
}

type nodeNetwork struct {
	field string
	cidr  string
}

// collectNodeNetworkCIDRs walks the provider configuration two levels deep — the fields live
// either at the top level or inside the layout section — and picks out the ones that name a
// network.
// document is the kind the fields are read from, so every message names the document the
// operator has to open rather than a bare field name that appears in several of them.
func collectNodeNetworkCIDRs(providerConfig map[string]json.RawMessage, document string) []nodeNetwork {
	var networks []nodeNetwork

	for key, raw := range providerConfig {
		if _, isNetwork := nodeNetworkCIDRFields[key]; isNetwork {
			var cidr string
			if err := json.Unmarshal(raw, &cidr); err == nil && cidr != "" {
				networks = append(networks, nodeNetwork{field: document + "." + key, cidr: cidr})
			}
			continue
		}

		var section map[string]json.RawMessage
		if err := json.Unmarshal(raw, &section); err != nil {
			continue
		}
		for nested, nestedRaw := range section {
			if _, isNetwork := nodeNetworkCIDRFields[nested]; !isNetwork {
				continue
			}
			var cidr string
			if err := json.Unmarshal(nestedRaw, &cidr); err == nil && cidr != "" {
				networks = append(networks, nodeNetwork{field: document + "." + key + "." + nested, cidr: cidr})
			}
		}
	}

	// Map iteration is unordered, and a check that reports its findings in a different order on
	// every run is hard to read and impossible to test.
	sort.Slice(networks, func(i, j int) bool { return networks[i].field < networks[j].field })
	return networks
}

func describeNetworks(networks []nodeNetwork) string {
	described := make([]string, 0, len(networks))
	for _, network := range networks {
		described = append(described, fmt.Sprintf("%s %s", network.field, network.cidr))
	}
	return strings.Join(described, ", ")
}

func CloudNodeNetworkCIDRIntersection(meta *config.MetaConfig) preflight.Check {
	check := CloudNodeNetworkCIDRIntersectionCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        CloudNodeNetworkCIDRIntersectionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Cacheable:   true,
		Run:         check.Run,
	}
}
