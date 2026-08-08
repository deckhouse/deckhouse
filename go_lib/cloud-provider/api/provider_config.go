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

package api

// ProviderClusterConfigObject is a typed providerClusterConfiguration usable by common rules.
//
// Implementations are instantiated with pointer types, so every provider-defined method
// must be nil-safe: the legacy resource is absent on a freshly bootstrapped cluster, and
// rules call these methods without checking for it first.
type ProviderClusterConfigObject interface {
	// HasMasterNodeGroup reports whether the masterNodeGroup section is set.
	HasMasterNodeGroup() bool
	// NodeGroupNames returns names of the additional node groups.
	NodeGroupNames() []string
}
