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

// This hook publishes the effective value of userAuthz.enableMultiTenancy
// (the ModuleConfig user settings already merged with config-schema
// defaults by addon-operator) into a ConfigMap.
//
// Why this exists: the multitenancy.py validating webhook has to know
// whether MultiTenancy is effectively enabled, but admission webhooks only
// see real Kubernetes objects, not module Values. Reading ModuleConfig
// directly is not enough — in some editions (e.g. CSE) enableMultiTenancy
// defaults to true in the config-schema, and when the user has not set it
// explicitly the field is simply absent from ModuleConfig.spec.settings.
// This hook runs on every module reconcile (OnBeforeHelm, so it fires
// whenever Values are recalculated — e.g. on a ModuleConfig update) and
// keeps the ConfigMap in sync with the real, defaults-merged value.
//
// The ConfigMap lives in this module's own d8-user-authz namespace, so its
// lifecycle stays tied to the module (cleaned up automatically along with
// everything else in that namespace when the module is disabled — a
// ConfigMap parked in some shared namespace like d8-system would just be
// orphaned there instead).
//
// The catch: d8-user-authz is itself only created by this chart when
// enableMultiTenancy is true (see templates/namespace.yaml) — so it's
// absent exactly when MultiTenancy is off, which is also the state we'd
// otherwise report by default. The Kubernetes binding below watches that
// namespace: if it's missing, there's nothing to publish (absence of the
// ConfigMap already reads as "disabled" to the webhook); once the chart
// creates it, the binding's own Kubernetes event re-runs this hook
// immediately, without waiting for the next unrelated module reconcile.
import (
	"context"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	MultitenancyStateConfigMapName = "d8-user-authz-multitenancy-state"
	MultitenancyStateDataKey       = "enableMultiTenancy"
	multitenancyNamespaceSnapshot  = "multitenancyNamespace"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/user-authz/discover-multitenancy-state",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       multitenancyNamespaceSnapshot,
			ApiVersion: "v1",
			Kind:       "Namespace",
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.Namespace},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return obj.GetName(), nil
			},
		},
	},
}, discoverMultitenancyState)

func discoverMultitenancyState(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get(multitenancyNamespaceSnapshot)) == 0 {
		// The module's own namespace doesn't exist yet — nothing to publish. The
		// webhook treats a missing ConfigMap the same as enableMultiTenancy=false,
		// which is correct: this namespace only exists when it's true.
		return nil
	}

	enabled := input.Values.Get("userAuthz.enableMultiTenancy").Bool()

	input.PatchCollector.CreateOrUpdate(&v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      MultitenancyStateConfigMapName,
			Namespace: internal.Namespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "user-authz",
			},
		},
		Data: map[string]string{
			MultitenancyStateDataKey: strconv.FormatBool(enabled),
		},
	})

	return nil
}
