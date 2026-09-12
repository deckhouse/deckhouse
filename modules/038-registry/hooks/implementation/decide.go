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

// Package implementation records whether this cluster may take a release that no longer carries the
// previous registry implementation.
//
// It lives in THIS release rather than in the one that removes the old implementation because a
// release requirement is evaluated by the version already installed: the release that declares
// `registryImplementation: V2` is judged by the code running here.
//
// The value means "an upgrade would not strand this cluster", not "the cluster runs the new
// implementation" — that code is not in this release. The two coincide everywhere except the case
// this package was written for: a `Direct` cluster whose operator has already written the new
// module's configuration, which is picked up on the other side of the upgrade.
package implementation

const (
	// ImplementationLegacy and ImplementationV2 are the two values a release can require, spelled
	// as in the release that consumes them, because they are compared as strings.
	ImplementationLegacy = "Legacy"
	ImplementationV2     = "V2"

	// LegacyStateSecretName is where the previous implementation persists its state machine.
	LegacyStateSecretName = "registry-state"
)

// legacyState is the part of the previous implementation's state this decision reads.
//
// Copied rather than imported, for the same reason the next release's gate copies them: the field
// names are the contract, and a rename shows up here as a mode that reads empty — which fails closed.
type legacyState struct {
	Mode       string `json:"mode,omitempty"`
	TargetMode string `json:"target_mode,omitempty"`
}

// decide answers whether an upgrade to a release without the previous implementation is safe.
//
// The question is not which implementation runs, but whether anything still depends on what the
// upgrade removes: the release on the far side does not render the previous implementation's objects
// — its Service, its in-cluster proxy — so a cluster whose nodes still point at the in-cluster
// address would be left with an address nobody serves.
//
//   - No state at all: the previous implementation never took the cluster.
//   - `Unmanaged`: it has already let go of the pull path, and the nodes pull from a registry named
//     in the configuration rather than from anything the upgrade deletes.
//   - `Direct` WITH the new module configured: the nodes do point at the in-cluster address, and its
//     replacement is already written down — after the upgrade the new implementation serves that
//     same address and the node agent takes over the container runtime configuration. The mode alone
//     is not enough, because without that configuration the address would be orphaned.
//   - Anything else, `Proxy` and `Local` included: refused. Both keep state the upgrade cannot
//     account for — static pods with their own PKI on every node, and, for `Local`, the image store
//     on the master disks.
//
// A transition in flight refuses in every case: a cluster being reconfigured right now is not a
// cluster to upgrade.
func decide(legacy *legacyState, moduleConfigured bool) string {
	if legacy == nil {
		return ImplementationV2
	}

	if legacy.TargetMode != "" && legacy.TargetMode != legacy.Mode {
		return ImplementationLegacy
	}

	switch legacy.Mode {
	case "Unmanaged":
		return ImplementationV2
	case "Direct":
		if moduleConfigured {
			return ImplementationV2
		}
	}

	return ImplementationLegacy
}
