/*
Copyright 2021 Flant JSC

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
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	carSnapshot = "cluster_authorization_rules"
)

type ClusterAuthorizationRule struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

func applyClusterAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	car := &ClusterAuthorizationRule{}
	car.Name = obj.GetName()
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return nil, fmt.Errorf(`".spec is not a map[string]interface{} or contains non-string values in the map: %s`, spew.Sdump(obj.Object))
	}
	if err != nil {
		return nil, err
	}

	if _, ok := spec["allowAccessToSystemNamespaces"]; ok {
		return nil, nil
	}

	if _, ok := spec["limitNamespaces"]; ok {
		return nil, nil
	}

	car.Spec = spec
	return car, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue(carSnapshot),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       carSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: applyClusterAuthorizationRuleFilter,
		},
	},
}, clusterAuthorizationRulesHandler)

func clusterAuthorizationRulesHandler(input *go_hook.HookInput) error {
	snapshots := input.Snapshots[carSnapshot]
	crds := make([]ClusterAuthorizationRule, 0)

	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		ccr := snapshot.(*ClusterAuthorizationRule)
		crds = append(crds, *ccr)
	}

	input.Values.Set("userAuthz.internal.crds", crds)

	return nil
}
