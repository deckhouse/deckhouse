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

import "fmt"

// Used when neither ModuleConfig nor ClusterConfiguration sets the domain.
const DefaultClusterDomain = "cluster.local"

func (m *MetaConfig) moduleConfigClusterDomain() string {
	return m.moduleConfigNetwork().ClusterDomain
}

func (m *MetaConfig) ClusterDomainResolved() string {
	if domain := m.moduleConfigClusterDomain(); domain != "" {
		return domain
	}
	if domain := m.clusterConfigString("clusterDomain"); domain != "" {
		return domain
	}
	return DefaultClusterDomain
}

// Set in both documents at once is unresolvable at bootstrap, even though ClusterDomainResolved
// would silently pick the ModuleConfig one.
func (m *MetaConfig) RequireClusterDomainSingleSource() error {
	if m.moduleConfigClusterDomain() != "" && m.clusterConfigString("clusterDomain") != "" {
		return fmt.Errorf(
			"clusterDomain must be set in only one of ModuleConfig control-plane-manager " +
				"(spec.settings.network.clusterDomain) or ClusterConfiguration (deprecated), not both")
	}
	return nil
}
