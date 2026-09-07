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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

// The second step of the removal of the per-custom-role bindings, see
// keep_legacy_custom_role_bindings.go: once the release has created the aggregated bindings, the
// old ones are deleted in parallel. The release engine has already let them go because of the keep
// annotation, so this is the only place they are removed from.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       internal.Queue("delete-legacy-custom-role-bindings"),
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(deleteLegacyCustomRoleBindings))

func deleteLegacyCustomRoleBindings(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	dynClient, err := newModuleBindingsClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	started := time.Now()

	deleted, err := deleteLegacyBindings(ctx, dynClient, legacyBindingsWorkers)
	if err != nil {
		return fmt.Errorf("delete legacy custom-role bindings: %w", err)
	}

	if deleted > 0 {
		input.Logger.Info("removed the per-custom-role bindings replaced by the aggregated ones",
			slog.Int("count", deleted), slog.Duration("took", time.Since(started)))
	}

	return nil
}

// deleteLegacyBindings removes every per-custom-role binding of the module and returns how many
// were deleted.
func deleteLegacyBindings(ctx context.Context, dynClient dynamic.Interface, workers int) (int, error) {
	return forEachRuleBindingParallel(ctx, dynClient, moduleBindingsSelector, isLegacyCustomRoleBinding, workers,
		func(ctx context.Context, ref bindingRef) error {
			err := dynClient.Resource(ref.gvr).Namespace(ref.namespace).Delete(ctx, ref.name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete %s %s/%s: %w", ref.gvr.Resource, ref.namespace, ref.name, err)
			}
			return nil
		})
}
