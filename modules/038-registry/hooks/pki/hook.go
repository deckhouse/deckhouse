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

package pki

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	valuesPath = "registry.internal.pki"
	queue      = "/modules/registry/pki"

	initPKISnap     = "init"
	registryPKISnap = "registry-pki"
	modulePKISnap   = "module-pki"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        queue,
		Kubernetes:   KubernetesConfigs(registryPKISnap, modulePKISnap, initPKISnap),
	},
	handle,
)

func handle(_ context.Context, input *go_hook.HookInput) error {
	values := helpers.NewValuesAccessor[Values](input, valuesPath)
	prev := values.Get()

	var inputs Inputs

	var initExists, initApplied bool
	if init, err := helpers.SnapshotToSingle[InitSnap](input, initPKISnap); err == nil {
		inputs.FromInit = &init.State
		initExists = true
		initApplied = init.Applied
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get registry-init snapshot: %w", err)
	}

	if reg, err := helpers.SnapshotToSingle[State](input, registryPKISnap); err == nil {
		inputs.FromRegistryPKI = &reg
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get registry-pki snapshot: %w", err)
	}

	var moduleInCluster bool
	var moduleSnap State
	if mod, err := helpers.SnapshotToSingle[State](input, modulePKISnap); err == nil {
		moduleSnap = mod
		moduleInCluster = true
		inputs.FromModulePKI = &mod
	} else if errors.Is(err, helpers.ErrNoSnapshot) {
		// Secret not created yet (first reconciles): reuse the last generated
		// material from values so the CA/users do not churn before round-trip.
		inputs.FromModulePKI = &prev.State
	} else {
		return fmt.Errorf("get registry-module-pki snapshot: %w", err)
	}

	var state State
	if _, err := state.Process(input.Logger, inputs); err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	hash, err := registry_pki.ComputeHash(state)
	if err != nil {
		return fmt.Errorf("cannot compute PKI hash: %w", err)
	}

	values.Set(Values{State: state, Hash: hash})

	markInitAppliedIfPersisted(input, inputs.FromInit, initExists, initApplied, moduleInCluster, moduleSnap)
	return nil
}

// markInitAppliedIfPersisted annotates the dhctl-seeded registry-init secret as
// applied once its CA has durably round-tripped into the in-cluster
// registry-module-pki, so dhctl's WaitForRegistryInitialization removes
// registry-init. In the orchestrator-free new arch the PKI hook is the only
// consumer that can signal this.
//
// The gate is deliberately strict: the CA must be present in the IN-CLUSTER
// registry-module-pki snapshot (not the values fallback used before the first
// round-trip) AND equal to the init CA. Annotating before persistence would let
// dhctl delete registry-init while registry-module-pki is still empty, losing
// the CA on the next hook restart and breaking node trust at the handoff.
func markInitAppliedIfPersisted(input *go_hook.HookInput, fromInit *State, initExists, initApplied, moduleInCluster bool, moduleSnap State) {
	if !shouldMarkInitApplied(fromInit, initExists, initApplied, moduleInCluster, moduleSnap) {
		return
	}
	input.Logger.Info("registry-init CA persisted in registry-module-pki; marking registry-init applied")
	input.PatchCollector.PatchWithMerge(
		map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					initSecretAppliedAnnotation: "",
				},
			},
		},
		"v1", "Secret", "d8-system", "registry-init")
}

// shouldMarkInitApplied is the continuity-critical gate (pure, unit-tested):
// mark registry-init applied ONLY when it exists, is not already applied, has a
// CA, and that exact CA is present in the IN-CLUSTER registry-module-pki
// snapshot. moduleInCluster distinguishes the real persisted secret from the
// values fallback used before the first round-trip.
func shouldMarkInitApplied(fromInit *State, initExists, initApplied, moduleInCluster bool, moduleSnap State) bool {
	if !initExists || initApplied {
		return false
	}
	if fromInit == nil || fromInit.CA == nil {
		return false
	}
	if !moduleInCluster || moduleSnap.CA == nil {
		return false
	}
	return moduleSnap.CA.Cert == fromInit.CA.Cert
}
