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
	"encoding/json"
	"fmt"
	"strings"
)

// Applied when the prefix is set in neither document. The two CIDRs have no such fallback — see
// RequireNetwork.
const DefaultPodSubnetNodeCIDRPrefix = "24"

// NetworkSettings is one resolved network parameter set. An empty field means "not set in this
// document", which is why none of the three may carry a schema default.
type NetworkSettings struct {
	PodSubnetCIDR           string
	ServiceSubnetCIDR       string
	PodSubnetNodeCIDRPrefix string
}

// networkParam is one parameter's candidate values, in precedence order.
type networkParam struct {
	name string
	mc   string
	cc   string
}

// networkParams enumerates the three parameters once, so the resolver, the "which document did it
// come from" report and the required-field check cannot drift apart.
func (m *MetaConfig) networkParams() []networkParam {
	mc := m.moduleConfigNetwork()
	cc := m.clusterConfigNetwork()

	return []networkParam{
		{"podSubnetCIDR", mc.PodSubnetCIDR, cc.PodSubnetCIDR},
		{"serviceSubnetCIDR", mc.ServiceSubnetCIDR, cc.ServiceSubnetCIDR},
		{"podSubnetNodeCIDRPrefix", mc.PodSubnetNodeCIDRPrefix, cc.PodSubnetNodeCIDRPrefix},
	}
}

// resolved returns the winning value: ModuleConfig if set there, else the deprecated field.
func (p networkParam) resolved() string {
	if p.mc != "" {
		return p.mc
	}
	return p.cc
}

// moduleConfigNetwork reads spec.settings.network off ModuleConfig control-plane-manager.
//
// Read raw, exactly like kubernetesVersionRaw: at bootstrap the settings-version conversion chain is
// not wired up, so a future conversion touching these keys must be reflected here or dhctl and
// admission (which sees converted settings) would disagree. The read is two-level and nil-safe: an
// absent network group is "nothing set in ModuleConfig", not an error. Non-strings are dropped
// rather than coerced.
func (m *MetaConfig) moduleConfigNetwork() NetworkSettings {
	out := NetworkSettings{}

	mc := m.FindModuleConfig("control-plane-manager")
	if mc == nil || mc.Spec.Settings == nil {
		return out
	}

	group, ok := mc.Spec.Settings["network"].(map[string]interface{})
	if !ok {
		return out
	}

	out.PodSubnetCIDR, _ = group["podSubnetCIDR"].(string)
	out.ServiceSubnetCIDR, _ = group["serviceSubnetCIDR"].(string)
	out.PodSubnetNodeCIDRPrefix, _ = group["podSubnetNodeCIDRPrefix"].(string)

	return out
}

// clusterConfigNetwork reads the three deprecated ClusterConfiguration fields. All three are
// optional now: the two CIDRs are no longer in the schema's required list and the prefix no longer
// carries a default, so a missing field is a normal state rather than an error.
func (m *MetaConfig) clusterConfigNetwork() NetworkSettings {
	return NetworkSettings{
		PodSubnetCIDR:           m.clusterConfigString("podSubnetCIDR"),
		ServiceSubnetCIDR:       m.clusterConfigString("serviceSubnetCIDR"),
		PodSubnetNodeCIDRPrefix: m.clusterConfigString("podSubnetNodeCIDRPrefix"),
	}
}

func (m *MetaConfig) clusterConfigString(key string) string {
	raw, ok := m.ClusterConfig[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return v
}

// Network returns the resolved network parameters: the ModuleConfig value when set, otherwise the
// deprecated ClusterConfiguration field. Each parameter is resolved independently — a half-migrated
// cluster with one value in each document is a state operators pass through.
//
// The prefix falls back to DefaultPodSubnetNodeCIDRPrefix. The CIDRs do not: there is no sane guess,
// and an empty string would be rendered into a master manifest. Use RequireNetwork to reject that
// before anything is rendered.
func (m *MetaConfig) Network() NetworkSettings {
	params := m.networkParams()

	out := NetworkSettings{
		PodSubnetCIDR:           params[0].resolved(),
		ServiceSubnetCIDR:       params[1].resolved(),
		PodSubnetNodeCIDRPrefix: params[2].resolved(),
	}

	if out.PodSubnetNodeCIDRPrefix == "" {
		out.PodSubnetNodeCIDRPrefix = DefaultPodSubnetNodeCIDRPrefix
	}

	return out
}

// NetworkFromClusterConfiguration lists the parameters whose value still comes from the deprecated
// ClusterConfiguration, for the bootstrap warning.
func (m *MetaConfig) NetworkFromClusterConfiguration() []string {
	var out []string
	for _, p := range m.networkParams() {
		if p.mc == "" && p.cc != "" {
			out = append(out, p.name)
		}
	}
	return out
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
