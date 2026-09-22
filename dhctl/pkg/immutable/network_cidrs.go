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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// providerInternalNetworkCIDRKeys names the field each cloud calls the network
// its nodes talk to each other over, by provider and then by the layout blocks
// that carry it. Providers absent from the table (DVP, zVirt) describe no such
// network at all.
//
// Mirrors the provider table of
// modules/040-node-manager/images/node-controller/src/internal/controller/nodeconfig/sources.go,
// which answers the same question for a node that is already in the cluster.
var providerInternalNetworkCIDRKeys = map[string][][]string{
	"openstack":   {{"standard", "internalNetworkCIDR"}, {"standardWithNoRouter", "internalNetworkCIDR"}},
	"huaweicloud": {{"standard", "internalNetworkCIDR"}, {"vpcPeering", "internalNetworkCIDR"}},
	"vsphere":     {{"internalNetworkCIDR"}},
	"vcd":         {{"internalNetworkCIDR"}},
	"yandex":      {{"nodeNetworkCIDR"}},
	"dynamix":     {{"nodeNetworkCIDR"}},
	"aws":         {{"nodeNetworkCIDR"}},
	"azure":       {{"subnetCIDR"}},
	"gcp":         {{"subnetworkCIDR"}},
}

// clusterInternalNetworkCIDRs are the networks the cluster reaches its nodes on.
// A static cluster names them itself; a cloud names one subnet, under a key of
// the provider's own choosing.
func clusterInternalNetworkCIDRs(metaConfig *config.MetaConfig) ([]string, error) {
	if metaConfig.ClusterType != config.CloudClusterType {
		return staticInternalNetworkCIDRs(metaConfig)
	}

	for _, path := range providerInternalNetworkCIDRKeys[metaConfig.ProviderName] {
		cidr, err := providerConfigString(metaConfig.ProviderClusterConfig, path)
		if err != nil {
			return nil, err
		}
		if cidr != "" {
			return []string{cidr}, nil
		}
	}
	return nil, nil
}

// staticInternalNetworkCIDRs reads what StaticClusterConfiguration names. Read
// the same way pkg/preflight/checks/cidr_intersection_static.go reads it: the
// field is optional, and a cluster without it leaves the choice to the node.
func staticInternalNetworkCIDRs(metaConfig *config.MetaConfig) ([]string, error) {
	raw, ok := metaConfig.StaticClusterConfig["internalNetworkCIDRs"]
	if !ok || len(raw) == 0 {
		return nil, nil
	}

	var cidrs []string
	if err := json.Unmarshal(raw, &cidrs); err != nil {
		return nil, fmt.Errorf("read internalNetworkCIDRs of the static cluster configuration: %w", err)
	}
	return cidrs, nil
}

// providerConfigString reads one string out of the provider configuration by
// path. A key the configuration does not carry is not an error: every layout
// block in the table is optional, and only one of them is ever filled in.
func providerConfigString(providerConfig map[string]json.RawMessage, path []string) (string, error) {
	level := providerConfig

	for _, key := range path[:len(path)-1] {
		raw, ok := level[key]
		if !ok || len(raw) == 0 {
			return "", nil
		}
		nested := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw, &nested); err != nil {
			return "", fmt.Errorf("read %s of the provider configuration: %w", key, err)
		}
		level = nested
	}

	raw, ok := level[path[len(path)-1]]
	if !ok || len(raw) == 0 {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("read %s of the provider configuration: %w", strings.Join(path, "."), err)
	}
	return value, nil
}
