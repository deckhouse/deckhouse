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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	v1alpha1 "github.com/deckhouse/deckhouse/modules/015-admission-policy-engine/hooks/internal/apis"
)

type operationPolicyFilterResult struct {
	Policy                 *operationPolicy `json:"policy"`
	ExplicitEmptySliceKeys []string         `json:"explicitEmptySliceKeys,omitempty"`
}

// operationPolicyEmptySlicePaths enumerates slice fields where explicit empty list ([]) must be
// preserved in Values so Helm `hasKey`-guards can distinguish "omitted" vs "explicitly empty".
//
// We keep it strictly to fields that participate in constraint rendering logic.
var operationPolicyEmptySlicePaths = []string{
	"spec.policies.allowedRepos",
	"spec.policies.requiredResources.limits",
	"spec.policies.requiredResources.requests",
	"spec.policies.disallowedImageTags",
	"spec.policies.disallowedTolerations",
	"spec.policies.requiredProbes",
	"spec.policies.priorityClassNames",
	"spec.policies.ingressClassNames",
	"spec.policies.storageClassNames",
	"spec.policies.requiredLabels.labels",
	"spec.policies.requiredLabels.watchKinds",
	"spec.policies.requiredAnnotations.annotations",
	"spec.policies.requiredAnnotations.watchKinds",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/admission-policy-engine/operation_policies",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "operation-policies",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "OperationPolicy",
			FilterFunc: filterOP,
		},
	},
}, handleOP)

func handleOP(_ context.Context, input *go_hook.HookInput) error {
	items, err := sdkobjectpatch.UnmarshalToStruct[operationPolicyFilterResult](input.Snapshots, "operation-policies")
	if err != nil {
		return fmt.Errorf("failed to unmarshal operation-policies snapshot: %w", err)
	}

	// We intentionally convert typed structs to map before putting them into Values to preserve
	// the semantic difference between:
	// - field omitted        => no key in Values (templates guarded by hasKey won't render)
	// - field set to []      => key exists in Values with empty array
	//
	// This is needed for selected fields (see filterOP) where explicitly empty list must not be
	// dropped by `omitempty` during JSON serialization.
	opsForValues := make([]map[string]any, 0, len(items))
	for _, item := range items {
		b, err := json.Marshal(item.Policy)
		if err != nil {
			return fmt.Errorf("failed to marshal OperationPolicy for Values: %w", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("failed to unmarshal OperationPolicy to map for Values: %w", err)
		}
		for _, key := range item.ExplicitEmptySliceKeys {
			path := strings.Split(key, ".")
			if err := unstructured.SetNestedField(m, []any{}, path...); err != nil {
				return fmt.Errorf("failed to force empty slice %q in Values: %w", key, err)
			}
		}
		opsForValues = append(opsForValues, m)
	}

	data, err := json.Marshal(opsForValues)
	if err != nil {
		return err
	}
	input.Values.Set("admissionPolicyEngine.internal.operationPolicies", json.RawMessage(data))

	return nil
}

func filterOP(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var op operationPolicy

	err := sdk.FromUnstructured(obj, &op)
	if err != nil {
		return nil, err
	}

	explicitEmpty, err := detectExplicitEmptySliceKeys(obj.Object, operationPolicyEmptySlicePaths)
	if err != nil {
		return nil, err
	}

	return &operationPolicyFilterResult{
		Policy:                 &op,
		ExplicitEmptySliceKeys: explicitEmpty,
	}, nil
}

type operationPolicy struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec v1alpha1.OperationPolicySpec `json:"spec"`
}
