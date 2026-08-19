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

// What the previous implementation leaves in the cluster, removed once this one owns it.
//
// Most of it needs no hook and gets none: everything that implementation created through Helm stops
// rendering the moment the handover happens, and Helm removes it on the same release. Measured on a
// migrated cluster — `registry-state`, `registry-pki`, the node configuration secrets, its Service and
// its DaemonSet were all already gone, with nothing asked of anyone.
//
// What remains is the handful of objects nobody owns in the Helm sense, written straight to the API by
// the installer and by bashible before any module ran. They have no owner to remove them and no reader
// left, so they stay until something says so. That is this hook.
//
// One-shot in effect rather than in machinery. It deletes what is present and says nothing about what
// is not, so running on every reconcile costs one no-op API call and the state it needs is the
// cluster's own. Nothing is remembered, so nothing can be remembered wrongly.
//
// # What is spared, and why
//
// Three names look like they belong on this list and do not:
//
//   - `registry-bashible-config` — this implementation's own. Both wrote it, under one name, and it is
//     how nodes are told about the registry right now.
//   - `deckhouse-registry` — still load-bearing. It is the imagePullSecret of this module's own
//     storage and controller, among others, so deleting it stops the very pods that serve the
//     registry. Removing the contour it belongs to is a platform-level decision (design ADR 22), not
//     something a module hook can do from inside.
//   - `registry-config` — rendered by module 002 on every cluster, and read by dhctl to resolve the
//     upstream when pushing a bundle. Deleting it from here would be a fight with the module that owns
//     it, and it would lose.
//
// The rule those three share: this hook removes what has no owner and no reader, and nothing else.
package v2

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

const (
	// InitSecretName is the secret the installer hands the first master its registry PKI in.
	//
	// Dead once a cluster is running, on either implementation: dhctl reads it back during
	// installation to build `deckhouse-registry` and never again — `registry.GetPKI` is called
	// only from the install path — and of the two implementations only the previous one ever read
	// it afterwards, to start its state machine. So on a cluster this implementation owns it is a
	// copy of registry credentials lying in the API with no reader, which is reason enough to
	// remove it beyond tidiness.
	InitSecretName = "registry-init"

	// LegacyPKISecretName and the state secret are Helm-owned and normally leave with the render
	// that stops declaring them. Deleted here as well because "normally" is doing work in that
	// sentence: a release that failed midway, or an object somebody annotated to survive, leaves
	// them behind, and a deletion of something already absent costs nothing.
	LegacyPKISecretName = "registry-pki"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// After the gate that decides whether this implementation is active at all: everything
		// here is conditional on that answer.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 7},
		Queue:        "/modules/registry/v2",
	},
	handleCleanup,
)

func handleCleanup(_ context.Context, input *go_hook.HookInput) error {
	if !IsActive(input) {
		// The previous implementation still owns the cluster, and every object below is either
		// its working state or something it will read. This is the one condition that matters:
		// there is no way back from the handover, so once it has happened these are dead for
		// good — but before it, they are alive.
		return nil
	}

	for _, name := range legacyLeftovers() {
		input.PatchCollector.Delete("v1", "Secret", "d8-system", name)
	}

	return nil
}

// legacyLeftovers names what is deleted, as a function so that the list itself can be asserted on.
//
// The assertion worth having is not that these three go, it is that nothing else joins them: the
// module's own secrets sit in the same namespace under similar names, and this list is the only thing
// standing between them and a deletion loop.
func legacyLeftovers() []string {
	return []string{
		InitSecretName,
		LegacyStateSecretName,
		LegacyPKISecretName,
	}
}
