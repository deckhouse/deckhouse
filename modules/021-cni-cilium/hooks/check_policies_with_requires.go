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

/*
Cilium 1.19 changed the type of fromRequires/toRequires to []string with MaxItems=0.
Since then the agent fails to decode the whole CNP/CCNP list if at least one policy still
has a non-empty value there, and exits after k8s-sync-timeout on every node.
This hook finds such policies, exposes a metric per policy and blocks the release
with Cilium 1.19 or later through the module requirement.
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	policiesWithRequiresMetricsGroup = "d8_cni_cilium_policies_with_requires"
	policiesWithRequiresMetricName   = "d8_cni_cilium_policy_with_requires_fields"
	// PoliciesWithRequiresRequirementKey is also read by the ciliumReadyForUpgradeTo requirement check.
	PoliciesWithRequiresRequirementKey = "cniCilium:policiesWithRequiresCount"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cni-cilium/policies-with-requires",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cnp",
			ApiVersion: "cilium.io/v2",
			Kind:       "CiliumNetworkPolicy",
			FilterFunc: filterPolicyWithRequires,
		},
		{
			Name:       "ccnp",
			ApiVersion: "cilium.io/v2",
			Kind:       "CiliumClusterwideNetworkPolicy",
			FilterFunc: filterPolicyWithRequires,
		},
	},
}, handlePoliciesWithRequires)

type policyWithRequires struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func filterPolicyWithRequires(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var specs []interface{}
	if spec, found, _ := unstructured.NestedMap(obj.Object, "spec"); found {
		specs = append(specs, spec)
	}
	if list, found, _ := unstructured.NestedSlice(obj.Object, "specs"); found {
		specs = append(specs, list...)
	}

	for _, s := range specs {
		spec, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		if hasNonEmptyField(spec, "fromRequires", "ingress", "ingressDeny") ||
			hasNonEmptyField(spec, "toRequires", "egress", "egressDeny") {
			return policyWithRequires{
				Kind:      obj.GetKind(),
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			}, nil
		}
	}

	return nil, nil
}

func hasNonEmptyField(spec map[string]interface{}, field string, ruleKeys ...string) bool {
	for _, key := range ruleKeys {
		rules, _, _ := unstructured.NestedSlice(spec, key)
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			if values, ok := rule[field].([]interface{}); ok && len(values) > 0 {
				return true
			}
		}
	}
	return false
}

func handlePoliciesWithRequires(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(policiesWithRequiresMetricsGroup)

	count := 0
	for _, snapshotName := range []string{"cnp", "ccnp"} {
		for policy, err := range sdkobjectpatch.SnapshotIter[policyWithRequires](input.Snapshots.Get(snapshotName)) {
			if err != nil {
				return fmt.Errorf("failed to iterate over %q snapshots: %w", snapshotName, err)
			}
			// Policies without the fields are filtered to nil and decode to the zero value.
			if policy.Name == "" {
				continue
			}

			count++
			input.MetricsCollector.Set(
				policiesWithRequiresMetricName,
				1,
				map[string]string{
					"kind":      policy.Kind,
					"namespace": policy.Namespace,
					"name":      policy.Name,
				},
				metrics.WithGroup(policiesWithRequiresMetricsGroup),
			)
		}
	}

	requirements.SaveValue(PoliciesWithRequiresRequirementKey, count)

	return nil
}
