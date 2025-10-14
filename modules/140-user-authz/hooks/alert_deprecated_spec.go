/*
Copyright 2023 Flant JSC

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

// this hook checks if there any clusterAuthorizationRules with limitNamespaces option set

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_authorization_rules",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: applyClusterAuthorizationRuleFilter,
		},
	},
}, handleClusterAuthorizationRulesWithDeprecatedSpec)

type ObjectCAR struct {
	Name       string
	Kind       string
	Deprecated bool
}

func applyClusterAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	car := ObjectCAR{
		Name: obj.GetName(),
		Kind: obj.GetKind(),
	}
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("couldn't find CAR spec")
	}

	if _, ok := spec["limitNamespaces"]; ok {
		car.Deprecated = true
	} else if _, ok := spec["allowAccessToSystemNamespaces"]; ok {
		car.Deprecated = true
	}

	return car, nil
}

func handleClusterAuthorizationRulesWithDeprecatedSpec(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_deprecated_car_spec")
	for car, err := range sdkobjectpatch.SnapshotIter[ObjectCAR](input.Snapshots.Get("cluster_authorization_rules")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'cluster_authorization_rules' snapshot: %w", err)
		}

		if car.Deprecated {
			input.MetricsCollector.Set("d8_deprecated_car_spec", 1, map[string]string{"kind": car.Kind, "name": car.Name}, metrics.WithGroup("d8_deprecated_car_spec"))
		}
	}
	return nil
}
