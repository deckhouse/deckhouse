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

// Resolution of clusterDomain inside the installer. Same precedence the global hook and
// node-controller implement; the code cannot be shared, the rule must not diverge.

package config

import (
	"cmp"
	"fmt"
)

// Used when neither ModuleConfig nor ClusterConfiguration sets the domain.
const DefaultClusterDomain = "cluster.local"

// One source for both candidate values, so the resolver and the two checks below cannot drift apart.
func (m *MetaConfig) clusterDomainParam() networkParam {
	return networkParam{
		name: "clusterDomain",
		mc:   m.moduleConfigNetwork().ClusterDomain,
		cc:   m.clusterConfigString("clusterDomain"),
	}
}

func (m *MetaConfig) ClusterDomainResolved() string {
	return cmp.Or(m.clusterDomainParam().resolved(), DefaultClusterDomain)
}

// ClusterDomainKnown is false only when the ModuleConfig could not be read and ClusterConfiguration
// carries no domain: resolving the default then would render a wrong service-account issuer.
func (m *MetaConfig) ClusterDomainKnown() bool {
	return !m.CPMModuleConfigUnreadable || m.clusterDomainParam().cc != ""
}

// Fails when the domain is set in neither document. Bootstrap and render only: the in-cluster hook
// parses without ModuleConfig documents, where this would reject every migrated cluster.
func (m *MetaConfig) RequireClusterDomain() error {
	if m.clusterDomainParam().resolved() != "" {
		return nil
	}

	return fmt.Errorf(
		"clusterDomain is not set: add spec.settings.network.clusterDomain to ModuleConfig " +
			"control-plane-manager (the deprecated ClusterConfiguration.clusterDomain is still " +
			"accepted, but raises a migration alert)")
}

// Set in both documents at once is unresolvable at bootstrap, even though ClusterDomainResolved
// would silently pick the ModuleConfig one.
func (m *MetaConfig) RequireClusterDomainSingleSource() error {
	if p := m.clusterDomainParam(); p.mc != "" && p.cc != "" {
		return fmt.Errorf(
			"clusterDomain must be set in only one of ModuleConfig control-plane-manager " +
				"(spec.settings.network.clusterDomain) or ClusterConfiguration (deprecated), not both")
	}
	return nil
}
