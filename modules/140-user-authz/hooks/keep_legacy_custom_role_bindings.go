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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

/*
The chart used to render one binding per custom ClusterRole for every rule,
user-authz:<rule>:<level>:custom-cluster-role:<role>. A rule now gets a single binding to the
aggregated role of its level, and the per-role bindings have left the manifest.

Their removal is done around the release engine, in two steps, so that no permission is lost:

 1. Before the release this hook stamps `helm.sh/resource-policy: keep` on them. The engine then
    plans nothing for these objects (nelm registers two operations for every object that leaves the
    manifest, delete and track absence, and the cost of building the plan grows with the square of
    the number of operations; a kept object costs one GET). The old bindings stay in the cluster
    and keep granting access while the release creates the aggregated ones.
 2. After the release has succeeded delete_legacy_custom_role_bindings.go deletes them in parallel.
    The aggregated bindings exist by then, so every permission the old bindings granted is already
    granted again.

Once the old bindings are gone both hooks list the module bindings and find nothing to do.
*/

const (
	// legacyCustomRoleBindingMarker is the part of the name that only the per-custom-role bindings had.
	legacyCustomRoleBindingMarker = ":custom-cluster-role:"

	legacyKeepAnnotation = "helm.sh/resource-policy"
	legacyKeepValue      = "keep"

	legacyBindingsWorkers = 16
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue("keep-legacy-custom-role-bindings"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(keepLegacyCustomRoleBindings))

func keepLegacyCustomRoleBindings(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	dynClient, err := newModuleBindingsClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	started := time.Now()

	stamped, err := stampKeepOnLegacyBindings(ctx, dynClient, legacyBindingsWorkers)
	if err != nil {
		return fmt.Errorf("stamp keep policy on legacy custom-role bindings: %w", err)
	}

	if err := verifyKeepOnLegacyBindings(ctx, dynClient); err != nil {
		return fmt.Errorf("verify keep policy on legacy custom-role bindings: %w", err)
	}

	if stamped > 0 {
		input.Logger.Info("protected the per-custom-role bindings from the release engine until their replacement is in place",
			slog.Int("count", stamped), slog.Duration("took", time.Since(started)))
	}

	return nil
}

// isLegacyCustomRoleBinding reports whether the binding is a per-custom-role binding of a rule.
func isLegacyCustomRoleBinding(obj *unstructured.Unstructured) bool {
	return isRuleBinding(obj) && strings.Contains(obj.GetName(), legacyCustomRoleBindingMarker)
}

func isLegacyCustomRoleBindingWithoutKeep(obj *unstructured.Unstructured) bool {
	return isLegacyCustomRoleBinding(obj) && obj.GetAnnotations()[legacyKeepAnnotation] != legacyKeepValue
}

// stampKeepOnLegacyBindings adds helm.sh/resource-policy: keep to every per-custom-role binding that
// does not have it yet and returns how many objects were patched.
func stampKeepOnLegacyBindings(ctx context.Context, dynClient dynamic.Interface, workers int) (int, error) {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				legacyKeepAnnotation: legacyKeepValue,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("marshal patch: %w", err)
	}

	return forEachRuleBindingParallel(ctx, dynClient, moduleBindingsSelector, isLegacyCustomRoleBindingWithoutKeep, workers,
		func(ctx context.Context, ref bindingRef) error {
			_, err := dynClient.Resource(ref.gvr).Namespace(ref.namespace).Patch(ctx, ref.name, types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("patch %s %s/%s: %w", ref.gvr.Resource, ref.namespace, ref.name, err)
			}
			return nil
		})
}

// verifyKeepOnLegacyBindings re-lists the per-custom-role bindings and fails if any still lacks the
// keep annotation: letting the release run would make the engine plan its deletion.
func verifyKeepOnLegacyBindings(ctx context.Context, dynClient dynamic.Interface) error {
	return forEachRuleBinding(ctx, dynClient, moduleBindingsSelector, isLegacyCustomRoleBindingWithoutKeep, func(ref bindingRef) error {
		return fmt.Errorf("keep policy is not set on %s %s/%s: refusing to proceed to the release", ref.gvr.Resource, ref.namespace, ref.name)
	})
}
