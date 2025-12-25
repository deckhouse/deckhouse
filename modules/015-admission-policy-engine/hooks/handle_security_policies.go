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
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses"
	"github.com/deckhouse/deckhouse/go_lib/set"
	v1alpha1 "github.com/deckhouse/deckhouse/modules/015-admission-policy-engine/hooks/internal/apis"
)

type securityPolicyFilterResult struct {
	Policy                 *securityPolicy `json:"policy"`
	ExplicitEmptySliceKeys []string        `json:"explicitEmptySliceKeys,omitempty"`
}

// securityPolicyEmptySlicePaths enumerates slice fields where explicit empty list ([]) must be
// preserved in Values so Helm `hasKey`-guards can distinguish "omitted" vs "explicitly empty".
//
// We keep it strictly to fields that participate in constraint rendering logic.
var securityPolicyEmptySlicePaths = []string{
	"spec.policies.allowedClusterRoles",
	"spec.policies.allowedFlexVolumes",
	"spec.policies.allowedVolumes",
	"spec.policies.allowedHostPaths",
	"spec.policies.allowedHostPorts",
	"spec.policies.allowedCapabilities",
	"spec.policies.requiredDropCapabilities",
	"spec.policies.allowedAppArmor",
	"spec.policies.allowedUnsafeSysctls",
	"spec.policies.forbiddenSysctls",
	"spec.policies.seLinux",
	"spec.policies.verifyImageSignatures",
	"spec.policies.allowedServiceTypes",
	// Nested: seccompProfiles is gated by `if $cr.spec.policies.seccompProfiles` and `hasKey` checks inside.
	"spec.policies.seccompProfiles.allowedProfiles",
	"spec.policies.seccompProfiles.allowedLocalhostFiles",
}

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

func handleSP(_ context.Context, input *go_hook.HookInput) error {
	items, err := sdkobjectpatch.UnmarshalToStruct[securityPolicyFilterResult](input.Snapshots, "security-policies")
	if err != nil {
		return fmt.Errorf("failed to unmarshal security-policies snapshot: %w", err)
	}

	refs := make(map[string]set.Set)

	for i := range items {
		item := &items[i]
		sp := item.Policy
		// set observed status
		input.PatchCollector.PatchWithMutatingFunc(
			set_cr_statuses.SetObservedStatus(item, filterSP),
			"deckhouse.io/v1alpha1",
			"securitypolicy",
			"",
			sp.Metadata.Name,
			object_patch.WithSubresource("/status"),
			object_patch.WithIgnoreHookError(),
		)
		preprocesSecurityPolicy(sp)

		for _, v := range sp.Spec.Policies.VerifyImageSignatures {
			if keys, ok := refs[v.Reference]; ok {
				for _, key := range v.PublicKeys {
					if !keys.Has(key) {
						keys.Add(key)
					}
				}
			} else {
				refs[v.Reference] = set.New(v.PublicKeys...)
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Policy.Metadata.Name < items[j].Policy.Metadata.Name
	})

	// Preserve explicit empty arrays for selected fields (see filterSP) while keeping the existing
	// behavior for all other fields (omitempty etc).
	policiesForValues := make([]map[string]any, 0, len(items))
	for _, item := range items {
		b, err := json.Marshal(item.Policy)
		if err != nil {
			return fmt.Errorf("failed to marshal SecurityPolicy for Values: %w", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("failed to unmarshal SecurityPolicy to map for Values: %w", err)
		}
		for _, key := range item.ExplicitEmptySliceKeys {
			path := strings.Split(key, ".")
			if err := unstructured.SetNestedField(m, []any{}, path...); err != nil {
				return fmt.Errorf("failed to force empty slice %q in Values: %w", key, err)
			}
		}
		policiesForValues = append(policiesForValues, m)
	}
	input.Values.Set("admissionPolicyEngine.internal.securityPolicies", policiesForValues)

	imageReferences := make([]ratifyReference, 0, len(refs))
	for k, v := range refs {
		imageReferences = append(imageReferences, ratifyReference{
			Reference:  k,
			PublicKeys: v.Slice(),
		})
	}

	sort.Slice(imageReferences, func(i, j int) bool {
		return imageReferences[i].Reference > imageReferences[j].Reference
	})
	input.Values.Set("admissionPolicyEngine.internal.ratify.imageReferences", imageReferences)

	return nil
}

func filterSP(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sp *securityPolicy
	err := sdk.FromUnstructured(obj, &sp)
	if err != nil {
		return nil, err
	}

	// Preserve semantic difference between omitted field and explicitly empty array for Helm templates
	// guarded by `hasKey`.
	explicitEmpty, err := detectExplicitEmptySliceKeys(obj.Object, securityPolicyEmptySlicePaths)
	if err != nil {
		return nil, err
	}

	return &securityPolicyFilterResult{
		Policy:                 sp,
		ExplicitEmptySliceKeys: explicitEmpty,
	}, nil
}

func detectExplicitEmptySliceKeys(obj map[string]any, dotPaths []string) ([]string, error) {
	out := make([]string, 0, len(dotPaths))
	for _, p := range dotPaths {
		path := strings.Split(p, ".")
		s, found, err := unstructured.NestedSlice(obj, path...)
		if err != nil {
			return nil, err
		}
		if found && len(s) == 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

func hasItem(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func preprocesSecurityPolicy(sp *securityPolicy) {
	// Check if we really need to create a constraint
	// AllowedCapabilities with 'ALL' and empty RequiredDropCapabilities list result in a sensless constraint
	if hasItem(sp.Spec.Policies.AllowedCapabilities, "ALL") && len(sp.Spec.Policies.RequiredDropCapabilities) == 0 {
		sp.Spec.Policies.AllowedCapabilities = nil
	}
	// AllowedUnsafeSysctls with '*' and empty ForbiddenSysctls list result in a sensless constraint
	if hasItem(sp.Spec.Policies.AllowedUnsafeSysctls, "*") && len(sp.Spec.Policies.ForbiddenSysctls) == 0 {
		sp.Spec.Policies.AllowedUnsafeSysctls = nil
	}
	// The rules set to 'RunAsAny' should be ignored
	if sp.Spec.Policies.FsGroup != nil {
		if sp.Spec.Policies.FsGroup.Rule == "RunAsAny" {
			sp.Spec.Policies.FsGroup = nil
		}
	}
	if sp.Spec.Policies.RunAsUser != nil {
		if sp.Spec.Policies.RunAsUser.Rule == "RunAsAny" {
			sp.Spec.Policies.RunAsUser = nil
		}
	}
	if sp.Spec.Policies.RunAsGroup != nil {
		if sp.Spec.Policies.RunAsGroup.Rule == "RunAsAny" {
			sp.Spec.Policies.RunAsGroup = nil
		}
	}
	if sp.Spec.Policies.SupplementalGroups != nil {
		if sp.Spec.Policies.SupplementalGroups.Rule == "RunAsAny" {
			sp.Spec.Policies.SupplementalGroups = nil
		}
	}
	if sp.Spec.Policies.AutomountServiceAccountToken == nil {
		sp.Spec.Policies.AutomountServiceAccountToken = ptr.To(true)
	}

	// 'Unmasked' procMount doesn't require a constraint
	if sp.Spec.Policies.AllowedProcMount == "Unmasked" {
		sp.Spec.Policies.AllowedProcMount = ""
	}
	// Having rules allowing '*' volumes makes no sense
	if hasItem(sp.Spec.Policies.AllowedVolumes, "*") {
		sp.Spec.Policies.AllowedVolumes = nil
	}
	// Having all seccomp profiles allowed also isn't worth creating a constraint
	if hasItem(sp.Spec.Policies.SeccompProfiles.AllowedProfiles, "*") && hasItem(sp.Spec.Policies.SeccompProfiles.AllowedLocalhostFiles, "*") {
		sp.Spec.Policies.SeccompProfiles.AllowedProfiles = nil
		sp.Spec.Policies.SeccompProfiles.AllowedLocalhostFiles = nil
	}
	// Having rules allowing '*' volumes makes no sense
	if hasItem(sp.Spec.Policies.AllowedClusterRoles, "*") {
		sp.Spec.Policies.AllowedClusterRoles = nil
	}
}

type securityPolicy struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec v1alpha1.SecurityPolicySpec `json:"spec"`
}

type ratifyReference struct {
	PublicKeys []string `json:"publicKeys"`
	Reference  string   `json:"reference"`
}
