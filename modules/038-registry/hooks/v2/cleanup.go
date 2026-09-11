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
// rendering the moment the handover happens, and Helm removes it on the same release.
//
// The objects still serving the pull path at that moment are the exception, held back deliberately:
// see the note on `registry-pki` below. What remains for this hook is the handful nobody owns in the
// Helm sense, written straight to the API by the installer and by bashible before any module ran —
// no owner to remove them and no reader left.
//
// One-shot in effect rather than in machinery: it deletes what is present and says nothing about what
// is not, so every later reconcile costs one no-op API call and the only state it needs is the
// cluster's own.
//
// Three names look like they belong on the list and do not:
//
//   - `registry-bashible-config` — this implementation's own. Both wrote it under one name, and it is
//     how nodes are told about the registry right now.
//   - `deckhouse-registry` — still the imagePullSecret of this module's own storage and controller,
//     so deleting it stops the very pods that serve the registry. Retiring the contour it belongs to
//     is a platform-level decision (design ADR 22), not a module hook's.
//   - `registry-config` — rendered by module 002 on every cluster and read by dhctl to resolve the
//     upstream when pushing a bundle. Deleting it here would be a fight with the module that owns it.
//
// The rule those three share: this hook removes what has no owner and no reader, and nothing else.
package v2

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
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

	// LegacyStateSecretName's companion, `registry-pki`, is NOT on this list, and that is the
	// one exclusion here that is about timing rather than ownership.
	//
	// It is the TLS material the legacy in-cluster proxy serves with, and that proxy is still
	// the pull path at this moment: this hook runs at the handover, while the nodes are still
	// configured for it and the agent that replaces it is not installed yet. Removing it here
	// took the fallback away exactly when it was still load-bearing — and it did so past the
	// `helm.sh/resource-policy: keep` the previous release puts on it for this reason, which
	// made the annotation look effective while it was being overruled from here.
	//
	// It is removed by the registry controller instead, which has the one fact this hook does
	// not: whether every node has the agent reconciled and listening. See
	// images/registry-controller/src/internal/controller/layout/legacy_cleanup.go.

	cleanupSwitchSnapName = "handover-recorded"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// After the gate that decides whether this implementation is active at all: everything
		// here is conditional on that answer.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 7},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				// The handover as the CLUSTER records it, which is not the same thing as the
				// decision taken by the gate on this pass — see deletionJustified.
				Name:       cleanupSwitchSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{SwitchSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					// Only its existence matters.
					return obj.GetName(), nil
				},
			},
		},
	},
	handleCleanup,
)

// deletionJustified answers whether what the previous implementation left may be removed.
//
// Two conditions. `active` is the decision this pass took: the previous implementation has let go of
// the pull path, so these objects have no reader. `recorded` is that handover being a fact rather
// than an intention — the marker secret exists, so a release of this implementation has succeeded at
// least once.
//
// Both, because the deletions are irreversible and run at OnBeforeHelm, before this pass has applied
// anything, the marker included. Keyed on the decision alone, the previous implementation's state
// goes out first while the release meant to replace it can still fail — for want of cert-manager, on
// quota, on admission — and what goes in that window includes the gate's own input, the legacy state
// secret it reads to decide.
//
// The cost of waiting is one reconciliation: the marker appears on the first successful release, so
// the installer's leftovers are removed on the pass after it.
func deletionJustified(active, recorded bool) bool {
	return active && recorded
}

func handleCleanup(_ context.Context, input *go_hook.HookInput) error {
	_, err := helpers.SnapshotToSingle[string](input, cleanupSwitchSnapName)
	recorded := err == nil

	if !deletionJustified(IsActive(input), recorded) {
		// Either the previous implementation still owns the cluster — every object below is then
		// its working state or something it will read — or the handover has not been recorded yet
		// and this pass may still fail with them already gone.
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
	}
}
