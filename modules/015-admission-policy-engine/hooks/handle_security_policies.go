/*
Copyright 2022 Flant JSC

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
	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1alpha1 "github.com/deckhouse/deckhouse/modules/015-admission-policy-engine/hooks/internal/apis"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/admission-policy-engine/security_policies",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "security-policies",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "SecurityPolicy",
			FilterFunc: filterSP,
		},
	},
}, handleSP)

func handleSP(input *go_hook.HookInput) error {
	result := make([]*securityPolicy, 0)

	snap := input.Snapshots["security-policies"]

	for _, sn := range snap {
		sp := sn.(*securityPolicy)
		result = append(result, sp)
	}

	data, _ := json.Marshal(result)

	input.Values.Set("admissionPolicyEngine.internal.securityPolicies", json.RawMessage(data))

	return nil
}

func filterSP(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sp securityPolicy

	err := sdk.FromUnstructured(obj, &sp)
	if err != nil {
		return nil, err
	}

	return &sp, nil
}

type securityPolicy struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec v1alpha1.SecurityPolicySpec `json:"spec"`
}
