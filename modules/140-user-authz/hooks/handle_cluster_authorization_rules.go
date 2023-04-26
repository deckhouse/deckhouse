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
	clusterAuthRuleSnapshot = "cluster_authorization_rules"
	authRuleSnapshot        = "authorization_rules"
)

type AuthorizationRule struct {
	Name      string                 `json:"name"`
	Spec      map[string]interface{} `json:"spec"`
	Namespace string                 `json:"namespace,omitempty"`
}

func applyAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return nil, fmt.Errorf(`".spec is not a map[string]interface{} or contains non-string values in the map: %s`, spew.Sdump(obj.Object))
	}
	if err != nil {
		return nil, err
	}

	car := &AuthorizationRule{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Spec:      spec,
	}

	return car, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("d8_auth_rules"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       clusterAuthRuleSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: applyAuthorizationRuleFilter,
		},
		{
			Name:       authRuleSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "AuthorizationRule",
			FilterFunc: applyAuthorizationRuleFilter,
		},
	},
}, authorizationRulesHandler)

func authorizationRulesHandler(input *go_hook.HookInput) error {
	input.Values.Set("userAuthz.internal.clusterAuthRuleCrds", snapshotsToAuthorizationRulesSlice(input.Snapshots[clusterAuthRuleSnapshot]))

	input.Values.Set("userAuthz.internal.authRuleCrds", snapshotsToAuthorizationRulesSlice(input.Snapshots[authRuleSnapshot]))

	return nil
}

func snapshotsToAuthorizationRulesSlice(snapshots []go_hook.FilterResult) []AuthorizationRule {
	ars := make([]AuthorizationRule, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		ar := snapshot.(*AuthorizationRule)
		ars = append(ars, *ar)
	}
	return ars
}
