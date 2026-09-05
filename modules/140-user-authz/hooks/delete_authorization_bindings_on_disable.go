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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

/*
The rule bindings are created by user-authz-controller and belong to no Helm release, so disabling
the module would leave them behind. Bindings to the module's own ClusterRoles go inert when those
roles are removed with the release, but a binding of additionalRoles points at a ClusterRole
outside the module and would keep granting access indefinitely. When the chart rendered the
bindings, the release uninstall removed them; this hook restores that behaviour after the
controller's Deployment is gone, so the two do not race.

The rules themselves stay (their CRDs are not removed), so their status is rewritten too: a
`Ready=True` left over from the controller would claim the bindings exist while they were just
removed. Re-enabling the module makes the controller recreate every binding from the rules and
report `Ready=True` again.
*/

const (
	deleteBindingsWorkers = 16

	conditionReady       = "Ready"
	reasonModuleDisabled = "ModuleDisabled"
)

var ruleResources = []schema.GroupVersionResource{
	{Group: "deckhouse.io", Version: "v1", Resource: "clusterauthorizationrules"},
	{Group: "deckhouse.io", Version: "v1alpha1", Resource: "authorizationrules"},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:             internal.Queue("delete-authorization-bindings"),
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(deleteAuthorizationBindingsOnDisable))

func deleteAuthorizationBindingsOnDisable(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	dynClient, err := newModuleBindingsClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	started := time.Now()

	deleted, err := deleteRuleBindings(ctx, dynClient, deleteBindingsWorkers)
	if err != nil {
		return fmt.Errorf("delete authorization bindings: %w", err)
	}

	marked, err := markRulesModuleDisabled(ctx, dynClient, deleteBindingsWorkers, time.Now())
	if err != nil {
		return fmt.Errorf("update the status of the authorization rules: %w", err)
	}

	input.Logger.Info("removed the authorization bindings of the disabled module",
		slog.Int("count", deleted), slog.Int("rules_marked", marked), slog.Duration("took", time.Since(started)))

	return nil
}

// markRulesModuleDisabled sets Ready=False/ModuleDisabled with zero bindings on every rule, so that
// the status does not keep claiming the bindings exist. Returns how many rules were patched.
func markRulesModuleDisabled(ctx context.Context, dynClient dynamic.Interface, workers int, now time.Time) (int, error) {
	total := 0
	var errs []error
	for _, gvr := range ruleResources {
		n, err := forEachResourceParallel(ctx, dynClient, gvr, workers, func(ctx context.Context, namespace, name string, generation int64) error {
			patch, err := json.Marshal(map[string]any{
				"status": map[string]any{
					"bindings":           0,
					"observedGeneration": generation,
					"conditions": []map[string]any{{
						"type":               conditionReady,
						"status":             "False",
						"reason":             reasonModuleDisabled,
						"message":            "the user-authz module is disabled; the bindings of the rule were removed",
						"observedGeneration": generation,
						"lastTransitionTime": now.UTC().Format(time.RFC3339),
					}},
				},
			})
			if err != nil {
				return fmt.Errorf("marshal status patch: %w", err)
			}
			_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{}, "status")
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("patch status of %s %s/%s: %w", gvr.Resource, namespace, name, err)
			}
			return nil
		})
		total += n
		if err != nil {
			errs = append(errs, err)
		}
	}
	return total, errors.Join(errs...)
}

// deleteRuleBindings removes every rule binding of the module and returns how many were deleted.
func deleteRuleBindings(ctx context.Context, dynClient dynamic.Interface, workers int) (int, error) {
	return forEachRuleBindingParallel(ctx, dynClient, moduleBindingsSelector, isRuleBinding, workers,
		func(ctx context.Context, ref bindingRef) error {
			err := dynClient.Resource(ref.gvr).Namespace(ref.namespace).Delete(ctx, ref.name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete %s %s/%s: %w", ref.gvr.Resource, ref.namespace, ref.name, err)
			}
			return nil
		})
}
