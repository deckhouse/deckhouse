/*
Copyright 2021 Flant CJSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/apis/audit"
	"sigs.k8s.io/yaml"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_audit_policy_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"audit-policy"},
			},
			FilterFunc: filterAuditSecret,
		},
	},
}, handleAuditPolicy)

func filterAuditSecret(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec v1.Secret

	err := sdk.FromUnstructured(unstructured, &sec)
	if err != nil {
		return nil, err
	}

	data := sec.Data["audit-policy.yaml"]

	return data, nil
}

func handleAuditPolicy(input *go_hook.HookInput) error {
	policyEnabled := input.Values.Get("controlPlaneManager.apiserver.auditPolicyEnabled")
	if !policyEnabled.Bool() {
		input.Values.Remove("controlPlaneManager.internal.auditPolicy")
		return nil
	}

	snap := input.Snapshots["kube_audit_policy_secret"]

	if len(snap) > 0 {
		data := snap[0].([]byte)

		var p audit.Policy
		err := yaml.UnmarshalStrict(data, &p)
		if err != nil {
			input.LogEntry.Errorf("invalid policy.yaml format: %s", err)
			return fmt.Errorf("invalid policy.yaml format")
		}

		input.Values.Set("controlPlaneManager.internal.auditPolicy", data)
	} else {
		input.Values.Remove("controlPlaneManager.internal.auditPolicy")
	}

	return nil
}
