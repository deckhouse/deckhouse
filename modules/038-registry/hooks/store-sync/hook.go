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

package storesync

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	storeSyncedPath = "registry.internal.takeover.storeSynced"
	queue           = "/modules/registry/store-sync"

	// hookOrder must be less than the takeover hook's Order 5 so that storeSynced
	// is written before the takeover hook's VerifyReady gate reads it.
	hookOrder = 3
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: hookOrder},
		Queue:        queue,
		Kubernetes:   []go_hook.KubernetesConfig{KubernetesConfig()},
	},
	handle,
)

func handle(_ context.Context, input *go_hook.HookInput) error {
	succeeded, err := helpers.SnapshotToSingle[int](input, storeSyncSnap)
	if err != nil {
		if errors.Is(err, helpers.ErrNoSnapshot) {
			// Job absent → not yet synced; set the leaf to false and return.
			input.Values.Set(storeSyncedPath, false)
			return nil
		}
		return fmt.Errorf("get store-sync Job snapshot: %w", err)
	}

	input.Values.Set(storeSyncedPath, jobSynced(succeeded))
	return nil
}

// jobSynced reports whether the store-sync Job has completed successfully.
// Pure function — no side effects, easy to test.
func jobSynced(succeeded int) bool {
	return succeeded > 0
}
