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

// Discovers the custom capabilities of the experimental role model that extend the namespace roles:
// every ClusterRole without heritage: deckhouse and without aggregationRule that carries the
// rbac.deckhouse.io/kind: use and rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL> labels.
// The d8:use:role:<LEVEL> roles aggregate such a capability through these labels. In the new label
// scheme they select rbac.deckhouse.io/aggregate-to-namespace-as instead, so a capability without
// that label adds nothing to them any more, and the subjects bound to them lose its permissions.
// A frozen custom role with the same labels extends them the same way and is counted too.
//
// Each such capability drives the D8UserAuthzLegacyRBACv2CustomCapabilityFound alert. A capability
// that already carries rbac.deckhouse.io/aggregate-to-namespace-as is not counted.

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	legacyCustomCapabilitiesSnapshot = "legacy_custom_capabilities"

	// legacyCustomCapabilityMetric drives the D8UserAuthzLegacyRBACv2CustomCapabilityFound alert: one
	// time series per capability, labelled with its name and the level it extends.
	legacyCustomCapabilityMetric = "d8_rbacv2_legacy_custom_capability"

	legacyUseAggregationLabel = "rbac.deckhouse.io/aggregate-to-kubernetes-as"
	namespaceAggregationLabel = "rbac.deckhouse.io/aggregate-to-namespace-as"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue(legacyCustomCapabilitiesSnapshot),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       legacyCustomCapabilitiesSnapshot,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyLegacyCustomCapabilityFilter,
		},
	},
}, discoveryLegacyCustomCapabilitiesHandler)

// legacyCustomCapability is what the snapshot keeps of a capability.
type legacyCustomCapability struct {
	Name  string `json:"name"`
	Level string `json:"level"`
}

// applyLegacyCustomCapabilityFilter keeps only the custom capabilities that extend a namespace role
// through the old labels alone.
func applyLegacyCustomCapabilityFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	role := new(rbacv1.ClusterRole)
	if err := sdk.FromUnstructured(obj, role); err != nil {
		return nil, err
	}
	level, ok := legacyCustomCapabilityLevel(role)
	if !ok {
		return nil, nil
	}
	return legacyCustomCapability{Name: role.Name, Level: level}, nil
}

// legacyCustomCapabilityLevel returns the level of the namespace role a custom capability extends
// through the old labels alone, and whether it does.
func legacyCustomCapabilityLevel(role *rbacv1.ClusterRole) (string, bool) {
	labels := role.Labels
	if labels["heritage"] == "deckhouse" || role.AggregationRule != nil || labels[rbacKindLabel] != "use" {
		return "", false
	}
	level := labels[legacyUseAggregationLabel]
	if level == "" {
		return "", false
	}
	if _, relabeled := labels[namespaceAggregationLabel]; relabeled {
		return "", false
	}
	return level, true
}

// discoveryLegacyCustomCapabilitiesHandler publishes one alert metric per capability, so the alert
// resolves as soon as a capability carries the new label or is deleted.
func discoveryLegacyCustomCapabilitiesHandler(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(legacyCustomCapabilityMetric)
	snapshot := input.Snapshots.Get(legacyCustomCapabilitiesSnapshot)
	for capability, err := range sdkobjectpatch.SnapshotIter[legacyCustomCapability](snapshot) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", legacyCustomCapabilitiesSnapshot, err)
		}
		input.MetricsCollector.Set(legacyCustomCapabilityMetric, 1,
			map[string]string{"name": capability.Name, "level": capability.Level},
			metrics.WithGroup(legacyCustomCapabilityMetric))
	}
	return nil
}
