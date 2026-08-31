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

// Resolution of the three cluster network parameters, which are being migrated from
// ClusterConfiguration into the network group of ModuleConfig control-plane-manager. Used by
// cluster_configuration.go, the only publisher of the resolved values.
//
// The values reach consumers through two key families, and both have to carry the resolved value:
//
//	global.discovery.podSubnet / .serviceSubnet / .podSubnetNodeCIDRPrefix
//	global.clusterConfiguration.{podSubnetCIDR,serviceSubnetCIDR,podSubnetNodeCIDRPrefix}
//
// Publishing into the second family is what keeps every template working unedited — including the
// snippet from the external deckhouse/lib-helm repo — and is the same substitution
// cluster_configuration.go already does for kubernetesVersion.

package hooks

import (
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	networkModuleConfigSnapshot = "controlPlaneManagerModuleConfigNetwork"

	// Applied when the parameter is set in neither document. The two CIDRs have no such fallback:
	// there is no sane guess for them, and inventing one would put an unrelated subnet into a master
	// manifest. Mirrors the constant the NodeGroup webhook has always used.
	defaultPodSubnetNodeCIDRPrefix = "24"
)

// networkSettings holds the raw network group of ModuleConfig control-plane-manager. An empty field
// means "not set there", which is what makes the ClusterConfiguration fallback reachable — hence no
// defaults on any of the three in the MC schema.
type networkSettings struct {
	PodSubnetCIDR           string `json:"podSubnetCIDR,omitempty"`
	ServiceSubnetCIDR       string `json:"serviceSubnetCIDR,omitempty"`
	PodSubnetNodeCIDRPrefix string `json:"podSubnetNodeCIDRPrefix,omitempty"`
}

// Never errors: an error would discard the snapshot and take down the only publisher of both key
// families. Reads spec.settings raw, with no settings-version conversion, so a conversion failure on
// an unrelated field cannot make a set parameter look unset.
func applyControlPlaneManagerNetworkFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	settings := networkSettings{}

	// Two-level and nil-safe: an absent network group is "nothing set here", not an error.
	group, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "spec", "settings", "network")
	if err != nil || !found {
		return settings, nil
	}

	settings.PodSubnetCIDR, _ = group["podSubnetCIDR"].(string)
	settings.ServiceSubnetCIDR, _ = group["serviceSubnetCIDR"].(string)
	// A non-string is dropped rather than coerced: the schema keeps this a string, and turning a
	// stray number into one here would launder an object that should be rejected.
	settings.PodSubnetNodeCIDRPrefix, _ = group["podSubnetNodeCIDRPrefix"].(string)

	return settings, nil
}

// resolvedNetwork is the outcome for one cluster: the values every consumer must agree on.
type resolvedNetwork struct {
	PodSubnetCIDR           string
	ServiceSubnetCIDR       string
	PodSubnetNodeCIDRPrefix string

	// Which of the values still come from the deprecated document, for the migration alert.
	FromClusterConfiguration []string
}

// resolveNetwork applies "ModuleConfig, otherwise ClusterConfiguration" to each parameter
// independently: a half-migrated cluster with one value in each document is a state operators pass
// through, and it has to work.
//
// Presence in ModuleConfig decides — the same rule as kubernetesVersion. There is no sentinel here,
// so presence is simply a non-empty value.
func resolveNetwork(mc networkSettings, cc networkSettings) resolvedNetwork {
	out := resolvedNetwork{}

	for _, p := range []struct {
		name string
		mc   string
		cc   string
		dst  *string
	}{
		{"podSubnetCIDR", mc.PodSubnetCIDR, cc.PodSubnetCIDR, &out.PodSubnetCIDR},
		{"serviceSubnetCIDR", mc.ServiceSubnetCIDR, cc.ServiceSubnetCIDR, &out.ServiceSubnetCIDR},
		{"podSubnetNodeCIDRPrefix", mc.PodSubnetNodeCIDRPrefix, cc.PodSubnetNodeCIDRPrefix, &out.PodSubnetNodeCIDRPrefix},
	} {
		switch {
		case p.mc != "":
			*p.dst = p.mc
		case p.cc != "":
			*p.dst = p.cc
			out.FromClusterConfiguration = append(out.FromClusterConfiguration, p.name)
		}
	}

	// Only the prefix has a fallback; see defaultPodSubnetNodeCIDRPrefix.
	if out.PodSubnetNodeCIDRPrefix == "" {
		out.PodSubnetNodeCIDRPrefix = defaultPodSubnetNodeCIDRPrefix
	}

	return out
}

// readNetworkModuleConfig is fail-open by construction: readSnapshot already degrades to the zero
// value on an unmarshal error, and the zero value means "nothing declared in ModuleConfig". This
// hook publishes both key families, so an unreadable ModuleConfig must never stop it — otherwise a
// malformed MC would erase podSubnet and serviceSubnet for the whole cluster.
func readNetworkModuleConfig(input *go_hook.HookInput) networkSettings {
	return readSnapshot[networkSettings](input, networkModuleConfigSnapshot)
}

func logNetworkFallback(input *go_hook.HookInput, resolved resolvedNetwork) {
	if len(resolved.FromClusterConfiguration) == 0 {
		return
	}

	input.Logger.Info(
		"cluster network parameters still come from the deprecated ClusterConfiguration; "+
			"move them into the network group of ModuleConfig control-plane-manager",
		slog.Any("parameters", resolved.FromClusterConfiguration),
	)
}
