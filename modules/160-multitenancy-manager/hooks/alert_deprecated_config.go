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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// allowNamespacesWithoutProjects is deprecated and ignored: every user namespace outside a project
// is adopted into a project of its own, whatever the parameter says. The schema keeps it for one
// release so an existing ModuleConfig still applies; the next minor release drops it from the
// schema, and a ModuleConfig that still carries it would then fail validation and stop applying.
// This hook is the warning in between: as long as the key is present in spec.settings -- with any
// value, since neither value does anything -- a metric is exported and an alert asks to remove it.
const (
	deprecatedConfigMetric      = "d8_mc_deprecated"
	deprecatedConfigMetricGroup = "d8_mc_multitenancy_manager"
	deprecatedConfigParameter   = "allowNamespacesWithoutProjects"
	moduleConfigSnapshot        = "module_config"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       moduleConfigSnapshot,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"multitenancy-manager"},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterDeprecatedConfigParameters,
		},
	},
}, alertOnDeprecatedConfigParameters)

// filterDeprecatedConfigParameters keeps only what the alert needs: the names of the deprecated
// parameters this ModuleConfig still sets. Presence is what matters, not the value.
func filterDeprecatedConfigParameters(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The name selector above already narrows the watch; the check is repeated here so the filter
	// stays correct on its own, and a same-named key in another module's config is never counted.
	if obj.GetName() != "multitenancy-manager" {
		return nil, nil
	}
	settings, _, err := unstructured.NestedMap(obj.Object, "spec", "settings")
	if err != nil {
		return nil, err
	}
	var present []string
	if _, ok := settings[deprecatedConfigParameter]; ok {
		present = append(present, deprecatedConfigParameter)
	}
	return present, nil
}

func alertOnDeprecatedConfigParameters(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(deprecatedConfigMetricGroup)

	for parameters, err := range sdkobjectpatch.SnapshotIter[[]string](input.Snapshots.Get(moduleConfigSnapshot)) {
		if err != nil {
			return fmt.Errorf("iterate over the '%s' snapshot: %w", moduleConfigSnapshot, err)
		}
		for _, parameter := range parameters {
			input.MetricsCollector.Set(deprecatedConfigMetric, 1, map[string]string{
				"module":    "multitenancy-manager",
				"parameter": parameter,
			}, metrics.WithGroup(deprecatedConfigMetricGroup))
		}
	}

	return nil
}
