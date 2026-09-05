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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

/*
The chart used to render one binding per custom ClusterRole for every rule,
user-authz:<rule>:<level>:custom-cluster-role:<role>. A rule now gets a single binding to the
aggregated role of its level, and the per-role bindings have left the manifest.

Their removal is not left to the release engine on purpose. nelm registers two operations for every
object that has to go (delete and track absence) and the cost of building the plan grows with the
square of the number of operations, so on a cluster with thousands of such bindings the release
would not fit into the release timeout, which is the very failure the aggregation fixes. Instead this
hook deletes them before the release, in parallel; the engine then only has to notice that the
objects are gone, one GET each and no operations at all.

The permissions granted through custom ClusterRoles are therefore unavailable between this hook and
the moment the release creates the aggregated bindings: seconds on an ordinary cluster, a few
minutes on a very large one. The access-level bindings of the rules are not touched.

Once the legacy bindings are gone the hook only lists the module bindings and finds nothing to do.
*/

const (
	// legacyCustomRoleBindingMarker is the part of the name that only the per-custom-role bindings had.
	legacyCustomRoleBindingMarker = ":custom-cluster-role:"

	deleteLegacyBindingsWorkers = 16
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue("delete-legacy-custom-role-bindings"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(deleteLegacyCustomRoleBindings))

func deleteLegacyCustomRoleBindings(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	dynClient, err := newModuleBindingsClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	started := time.Now()

	deleted, err := deleteLegacyBindings(ctx, dynClient, deleteLegacyBindingsWorkers)
	if err != nil {
		return fmt.Errorf("delete legacy custom-role bindings: %w", err)
	}

	if deleted > 0 {
		input.Logger.Info("removed the per-custom-role bindings replaced by the aggregated ones",
			slog.Int("count", deleted), slog.Duration("took", time.Since(started)))
	}

	return nil
}

// isLegacyCustomRoleBinding reports whether the binding is a per-custom-role binding of a rule.
func isLegacyCustomRoleBinding(obj *unstructured.Unstructured) bool {
	return isRuleBinding(obj) && strings.Contains(obj.GetName(), legacyCustomRoleBindingMarker)
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
