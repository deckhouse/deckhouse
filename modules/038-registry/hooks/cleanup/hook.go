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

package cleanup

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	queue     = "/modules/registry/cleanup"
	hookOrder = 20
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: hookOrder},
		Queue:        queue,
		Kubernetes:   KubernetesConfigs(),
	},
	handle,
)

func handle(_ context.Context, input *go_hook.HookInput) error {
	if !shouldClean(helpers.TakeoverPhase(input)) {
		return nil
	}

	// Delete registry-config if present
	names, err := helpers.SnapshotToList[string](input, legacyConfigSnap)
	if err != nil {
		return fmt.Errorf("get legacy-config snapshot: %w", err)
	}
	for _, name := range names {
		input.PatchCollector.Delete("v1", "Secret", "d8-system", name)
	}

	// Delete per-node registry-node-config-* secrets
	nodeNames, err := helpers.SnapshotToList[string](input, nodeConfigSnap)
	if err != nil {
		return fmt.Errorf("get node-config snapshot: %w", err)
	}
	for _, name := range nodeNames {
		input.PatchCollector.Delete("v1", "Secret", "d8-system", name)
	}

	// CA guard: only delete registry-pki/registry-state if module CA is durable
	caPresent, err := helpers.SnapshotToSingle[bool](input, modulePKISnap)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get module-pki snapshot: %w", err)
	}

	if caDurable(caPresent) {
		stateNames, err := helpers.SnapshotToList[string](input, stateSnap)
		if err != nil {
			return fmt.Errorf("get registry-state snapshot: %w", err)
		}
		for _, name := range stateNames {
			input.PatchCollector.Delete("v1", "Secret", "d8-system", name)
		}

		pkiNames, err := helpers.SnapshotToList[string](input, legacyPKISnap)
		if err != nil {
			return fmt.Errorf("get legacy-pki snapshot: %w", err)
		}
		for _, name := range pkiNames {
			input.PatchCollector.Delete("v1", "Secret", "d8-system", name)
		}
	}

	return nil
}

func shouldClean(phase string) bool { return phase == helpers.PhaseCleanupPending }

// caDurable gates PKI-related deletion: only drop legacy registry-pki once the
// module CA is confirmed present in registry-module-pki.
func caDurable(modulePKICAPresent bool) bool { return modulePKICAPresent }
