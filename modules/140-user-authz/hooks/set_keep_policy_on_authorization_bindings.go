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
The bindings of ClusterAuthorizationRules and AuthorizationRules used to be rendered by the module
chart and are now owned by user-authz-controller. On the release that drops them from the chart the
release engine would prune every binding it rendered before — and the controller, deployed by that
same release, cannot be running yet to recreate them. Both engines honour the
`helm.sh/resource-policy: keep` annotation on the live object during an upgrade, so this hook stamps
it on every Helm-managed binding of the module right before the release runs, then verifies and
refuses to let the release proceed if any binding is left unprotected (the same pattern
node-manager used when it moved its machine objects into node-controller).

Once the controller adopts a binding it labels it as managed by the controller and drops the
annotation, so the hook becomes a no-op: the selector below skips the adopted bindings. Both release
engines put `app.kubernetes.io/managed-by: Helm` on the objects they apply, but the selector keys on
the controller's own label instead: that is what decides ownership here, and stamping a chart-era
binding the engine happens not to track is harmless while missing one is not.

Engine notes: nelm reads the policy from the live object and also refuses to delete an object whose
ownership metadata no longer matches the release, so an adoption racing the release is safe too.
helm3 reads the live annotation on upgrade (the path taken here) but the stored manifest on
uninstall and rollback; neither of those is a supported operation for a Deckhouse module release.

The hook uses its own client with a raised rate limit: the shared hook client is capped at the
client-go default of 5 QPS, which for tens of thousands of bindings would block the module
converge for an hour. Lists are paged so that the hook never holds every binding in memory at once.
*/

const (
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	helmResourcePolicyKeep       = "keep"

	// unadoptedBindingsSelector matches the bindings the chart rendered and the controller has not
	// adopted yet, whichever release engine rendered them.
	unadoptedBindingsSelector = "heritage=deckhouse,module=user-authz,!user-authz.deckhouse.io/managed-by"

	keepPolicyWorkers = 16
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue("keep-policy-authorization-bindings"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(setKeepPolicyOnAuthorizationBindings))

func setKeepPolicyOnAuthorizationBindings(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	dynClient, err := newModuleBindingsClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	started := time.Now()

	stamped, err := stampKeepPolicy(ctx, dynClient, keepPolicyWorkers)
	if err != nil {
		return fmt.Errorf("stamp keep policy on authorization bindings: %w", err)
	}

	if err := verifyKeepPolicy(ctx, dynClient); err != nil {
		return fmt.Errorf("verify keep policy on authorization bindings: %w", err)
	}

	if stamped > 0 {
		input.Logger.Info("stamped keep policy on authorization bindings rendered by the chart",
			slog.Int("count", stamped), slog.Duration("took", time.Since(started)))
	}

	return nil
}

// stampKeepPolicy adds helm.sh/resource-policy: keep to every Helm-managed rule binding that does not
// have it yet and returns how many objects were patched.
func stampKeepPolicy(ctx context.Context, dynClient dynamic.Interface, workers int) (int, error) {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				helmResourcePolicyAnnotation: helmResourcePolicyKeep,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("marshal patch: %w", err)
	}

	return forEachRuleBindingParallel(ctx, dynClient, unadoptedBindingsSelector, isRuleBindingWithoutKeep, workers,
		func(ctx context.Context, ref bindingRef) error {
			_, err := dynClient.Resource(ref.gvr).Namespace(ref.namespace).Patch(ctx, ref.name, types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				// a binding deleted since the list (its rule is gone) needs no protection
				return fmt.Errorf("patch %s %s/%s: %w", ref.gvr.Resource, ref.namespace, ref.name, err)
			}
			return nil
		})
}

// verifyKeepPolicy re-lists the Helm-managed rule bindings and fails if any still lacks the keep
// annotation: letting the release run would prune it.
func verifyKeepPolicy(ctx context.Context, dynClient dynamic.Interface) error {
	return forEachRuleBinding(ctx, dynClient, unadoptedBindingsSelector, isRuleBindingWithoutKeep, func(ref bindingRef) error {
		return fmt.Errorf("keep policy is not set on %s %s/%s: refusing to proceed to avoid prune", ref.gvr.Resource, ref.namespace, ref.name)
	})
}

func isRuleBindingWithoutKeep(obj *unstructured.Unstructured) bool {
	if !isRuleBinding(obj) {
		return false
	}

	return obj.GetAnnotations()[helmResourcePolicyAnnotation] != helmResourcePolicyKeep
}
