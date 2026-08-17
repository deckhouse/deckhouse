/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	bashible_model "github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/v2/pki"
)

// valuesPath is where the internal values of this implementation live.
const valuesPath = "registry.internal.v2"

// Values are all the internal values of the controller-based implementation.
//
// One struct for every hook in this package, deliberately. The values accessor reads
// and writes the whole object at `valuesPath`, so a hook holding a struct with only
// its own fields would silently drop every other hook's on write — the values would
// depend on which hook ran last, which is the kind of failure that only shows up in a
// cluster.
type Values struct {
	// Enabled reports that this implementation is active. Every template of it is
	// rendered under this, so both implementations owning the pull path at once is
	// impossible by construction.
	Enabled bool `json:"enabled"`

	// SwitchBlockedReason is why the switch was refused while ModuleConfig asks for
	// it. Empty when nothing is blocking.
	SwitchBlockedReason string `json:"switchBlockedReason,omitempty"`

	// PKI is the storage certificate material, generated once and persisted.
	PKI *pki.State `json:"pki,omitempty"`

	// StorageAddresses are the addresses the storage replicas are reachable at.
	//
	// Node addresses, not the Service name. Everything that talks to the storage from a
	// node — the agent pulling, a replica replicating — runs in the host network and
	// resolves through the host's resolver, where a cluster DNS name does not exist. The
	// Service name is only ever a key that names the image set.
	//
	// The same set the serving certificate is issued for, which is why both come from
	// one place.
	StorageAddresses []string `json:"storageAddresses,omitempty"`

	// RegistryConfig is the resolved primary configuration the RegistryConfig
	// resource is rendered from.
	RegistryConfig *RegistryConfig `json:"registryConfig,omitempty"`

	// BashibleConfig is what nodes are told, including the marker that hands the
	// container runtime's registry configuration to the node agent.
	BashibleConfig *bashible_model.Config `json:"bashibleConfig,omitempty"`

	// ImageAddress is the address container image references may be rendered from,
	// empty until every node's agent is applying the layout the cluster gave it.
	//
	// Nodes have been told this address all along — it is `ImagesBase` in
	// BashibleConfig — but a node learns it through a bashible rollout, while the
	// platform renders every image reference in one pass. This is what keeps the
	// second from running ahead of the first.
	ImageAddress string `json:"imageAddress,omitempty"`
}

func accessor(input *go_hook.HookInput) helpers.ValuesAccessor[Values] {
	return helpers.NewValuesAccessor[Values](input, valuesPath)
}

// IsActive reports that this implementation owns the cluster's registry.
//
// Exported for the legacy implementation to read, and that is the only import between the
// two packages. The direction is deliberate: the legacy hooks may depend on this one, so
// removing them stays a single deletion, whereas a dependency the other way would be the
// thread left holding them together.
//
// Safe to call from any hook ordered after the switch (OnBeforeHelm order 5), which is
// where the value is written. Before that it reads false — the same answer as "the previous
// implementation owns the cluster", which is the answer that keeps the legacy state machine
// running rather than the one that starts a second writer.
func IsActive(input *go_hook.HookInput) bool {
	return accessor(input).Get().Enabled
}
