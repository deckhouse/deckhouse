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

// Resolution of the three cluster network parameters inside the installer. They are being migrated
// from ClusterConfiguration into the network group of ModuleConfig control-plane-manager, and the
// values have to work from the very first bootstrap step, long before the ModuleConfig API exists:
// dhctl reads them out of the ModuleConfig documents in config.yml.
//
// The precedence is the same one global-hooks and node-controller implement, but the code cannot be
// shared — the three runtimes have nothing in common. Keeping the rule identical in all three is
// what stops the node subnet mask in kube-controller-manager from diverging from the pod limit
// derived from it.

package config

import (
	"cmp"
	"fmt"
	"strings"
)

// Applied when the prefix is set in neither document. The two CIDRs have no such fallback — see
// RequireNetwork.
const DefaultPodSubnetNodeCIDRPrefix = "24"

// NetworkSettings is one resolved network parameter set. An empty field means "not set in this
// document", which is why neither of the two CIDR fields may carry a schema default.
type NetworkSettings struct {
	PodSubnetCIDR           string
	ServiceSubnetCIDR       string
	PodSubnetNodeCIDRPrefix string
	ClusterDomain           string
}

// networkSettings are the settings RequireNetworkSingleSource and the network checks walk.
var networkSettings = []clusterSetting{
	clusterSettingPodSubnetCIDR,
	clusterSettingServiceSubnetCIDR,
	clusterSettingPodSubnetNodeCIDRPrefix,
	clusterSettingClusterDomain,
}

// Network returns the resolved network parameters: the ModuleConfig value when set, otherwise the
// deprecated ClusterConfiguration field. Each parameter is resolved independently — a half-migrated
// cluster with one value in each document is a state operators pass through.
//
// The prefix falls back to DefaultPodSubnetNodeCIDRPrefix. The CIDRs do not: there is no sane guess,
// and an empty string would be rendered into a master manifest. Use RequireNetwork to reject that
// before anything is rendered.
func (m *MetaConfig) Network() NetworkSettings {
	return NetworkSettings{
		PodSubnetCIDR:           m.setting(clusterSettingPodSubnetCIDR).value(),
		ServiceSubnetCIDR:       m.setting(clusterSettingServiceSubnetCIDR).value(),
		PodSubnetNodeCIDRPrefix: cmp.Or(m.setting(clusterSettingPodSubnetNodeCIDRPrefix).value(), DefaultPodSubnetNodeCIDRPrefix),
		ClusterDomain:           m.ClusterDomainResolved(),
	}
}

// RequireNetworkSingleSource fails when a parameter is set in both documents at once. A
// half-migrated cluster with different values in each document is impossible to resolve
// unambiguously at bootstrap, even though Network() would silently pick the ModuleConfig one.
func (m *MetaConfig) RequireNetworkSingleSource() error {
	var both []string
	for _, name := range networkSettings {
		candidates := m.setting(name)
		if candidates.fromModuleConfig != "" && candidates.fromClusterConfig != "" {
			both = append(both, string(name))
		}
	}

	if len(both) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%s must be set in only one of ModuleConfig control-plane-manager (spec.settings.network) "+
			"or ClusterConfiguration (deprecated), not both",
		strings.Join(both, " and "),
	)
}

// RequireNetwork fails when either CIDR is set in neither document.
//
// This obligation used to belong to the ClusterConfiguration schema, which listed both CIDRs as
// required. Removing them from that list is what lets a cluster bootstrap with the values only in
// ModuleConfig, and it moves the check here — the installer must fail while parsing config.yml
// rather than render an empty --service-cluster-ip-range into a master manifest.
//
// Deliberately NOT called from Prepare, even though Prepare is where all parse paths converge: the
// in-cluster hook reaches Prepare through ParseConfigFromData with only the Secret contents and no
// ModuleConfig documents at all. For that caller ModuleConfigs is always empty, so this check would
// reject exactly the clusters that have already migrated. It belongs to bootstrap only.
func (m *MetaConfig) RequireNetwork() error {
	network := m.Network()

	var missing []string
	if network.PodSubnetCIDR == "" {
		missing = append(missing, "podSubnetCIDR")
	}
	if network.ServiceSubnetCIDR == "" {
		missing = append(missing, "serviceSubnetCIDR")
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%s must be set either in ModuleConfig control-plane-manager (spec.settings.network) "+
			"or in ClusterConfiguration (deprecated)",
		strings.Join(missing, " and "),
	)
}
