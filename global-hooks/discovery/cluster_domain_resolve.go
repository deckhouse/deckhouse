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

// Resolution of clusterDomain: ModuleConfig control-plane-manager takes precedence over the
// deprecated ClusterConfiguration. Used by cluster_configuration.go, the only publisher of the value.

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

const clusterDomainModuleConfigSnapshot = "controlPlaneManagerModuleConfigClusterDomain"

// Never errors: that would discard the snapshot and leave the cluster without a domain. Reads raw, so
// a conversion failure elsewhere cannot make a set domain look unset.
func applyControlPlaneManagerClusterDomainFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// A missing key and a wrong type both yield "", so neither needs a branch of its own.
	domain, _, _ := unstructured.NestedString(obj.UnstructuredContent(), "spec", "settings", "network", "clusterDomain")
	return domain, nil
}

// fromClusterConfiguration reports whether the deprecated document still decides, for the log line.
func resolveClusterDomain(mc string, cc string) (domain string, fromClusterConfiguration bool) {
	switch {
	case mc != "":
		return mc, false
	case cc != "":
		return cc, true
	default:
		return hooks.DefaultClusterDomain, false
	}
}

// Unreadable ModuleConfig reads as "not set here" - it must never erase the domain.
func readClusterDomainModuleConfig(input *go_hook.HookInput) string {
	return readSnapshot[string](input, clusterDomainModuleConfigSnapshot)
}
