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
	"fmt"
	"log/slog"
	"slices"
	"sort"

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
	policies, err := sdkobjectpatch.UnmarshalToStruct[securityPolicy](input.Snapshots, "security-policies")
	if err != nil {
		return fmt.Errorf("failed to unmarshal security-policies snapshot: %w", err)
	}

	refs := make(map[string]set.Set)

	// A user policy that already holds the name of a published standard keeps it: the standards
	// are published only to describe them, so the module leaves that standard unpublished rather
	// than fail its release on an object it cannot adopt.
	conflicts := make([]string, 0)
	for _, sp := range policies {
		if slices.Contains(publishedStandardNames, sp.Metadata.Name) && !isPodSecurityStandard(sp) {
			conflicts = append(conflicts, sp.Metadata.Name)
			input.Logger.Warn("a SecurityPolicy holds the name of a published Pod Security Standard, the standard is not published",
				slog.String("name", sp.Metadata.Name))
		}
	}
	sort.Strings(conflicts)
	input.Values.Set("admissionPolicyEngine.internal.podSecurityStandards.conflictingPolicyNames", conflicts)

	policies = slices.DeleteFunc(policies, isPodSecurityStandard)

	for i := range policies {
		sp := &policies[i]
		// set observed status
		input.PatchCollector.PatchWithMutatingFunc(
			set_cr_statuses.SetObservedStatus(sp, filterSP),
			"deckhouse.io/v1alpha1",
			"securitypolicy",
			"",
			sp.Metadata.Name,
			object_patch.WithSubresource("/status"),
			object_patch.WithIgnoreHookError(),
		)
		preprocesSecurityPolicy(sp)

		if sp.Spec.Policies.VerifyImageSignatures != nil {
			for _, v := range *sp.Spec.Policies.VerifyImageSignatures {
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
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Metadata.Name < policies[j].Metadata.Name
	})
	input.Values.Set("admissionPolicyEngine.internal.securityPolicies", policies)

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

	return sp, nil
}

// isPodSecurityStandard reports whether the policy is one of the Pod Security Standards the module
// publishes as SecurityPolicy objects. They describe the standards only: the module renders the
// constraints of the standards itself, with a namespace selection no SecurityPolicy can express,
// so rendering them as user policies would enforce every rule twice and in the wrong namespaces.
//
// The `heritage: deckhouse` label cannot be set by a user, so a user policy is never taken for one.
func isPodSecurityStandard(sp securityPolicy) bool {
	labels := sp.Metadata.Labels
	if labels["heritage"] != "deckhouse" {
		return false
	}
	_, ok := labels[podStandardLabel]
	return ok
}

func hasItem(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func hasItemPtr(slice *[]string, value string) bool {
	if slice == nil {
		return false
	}
	for _, v := range *slice {
		if v == value {
			return true
		}
	}
	return false
}

func sliceLenPtr[T any](slice *[]T) int {
	if slice == nil {
		return 0
	}
	return len(*slice)
}

func preprocesSecurityPolicy(sp *securityPolicy) {
	// Check if we really need to create a constraint
	// AllowedCapabilities with 'ALL' and empty RequiredDropCapabilities list result in a sensless constraint
	if hasItemPtr(sp.Spec.Policies.AllowedCapabilities, "ALL") && sliceLenPtr(sp.Spec.Policies.RequiredDropCapabilities) == 0 {
		sp.Spec.Policies.AllowedCapabilities = nil
	}
	// AllowedUnsafeSysctls with '*' and empty ForbiddenSysctls list result in a sensless constraint
	if hasItemPtr(sp.Spec.Policies.AllowedUnsafeSysctls, "*") && sliceLenPtr(sp.Spec.Policies.ForbiddenSysctls) == 0 {
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
	if hasItemPtr(sp.Spec.Policies.AllowedVolumes, "*") {
		sp.Spec.Policies.AllowedVolumes = nil
	}
	// Having all seccomp profiles allowed also isn't worth creating a constraint
	if sp.Spec.Policies.SeccompProfiles != nil {
		if hasItemPtr(sp.Spec.Policies.SeccompProfiles.AllowedProfiles, "*") && hasItemPtr(sp.Spec.Policies.SeccompProfiles.AllowedLocalhostFiles, "*") {
			sp.Spec.Policies.SeccompProfiles.AllowedProfiles = nil
			sp.Spec.Policies.SeccompProfiles.AllowedLocalhostFiles = nil
		}
		if sp.Spec.Policies.SeccompProfiles.AllowedProfiles == nil && sp.Spec.Policies.SeccompProfiles.AllowedLocalhostFiles == nil {
			sp.Spec.Policies.SeccompProfiles = nil
		}
	}
	// Having rules allowing '*' volumes makes no sense
	if hasItemPtr(sp.Spec.Policies.AllowedClusterRoles, "*") {
		sp.Spec.Policies.AllowedClusterRoles = nil
	}
}

const podStandardLabel = "security.deckhouse.io/pod-standard"

// publishedStandardNames are the SecurityPolicy objects the module publishes the Pod Security
// Standards as, see templates/policies/pod-security-standards.yaml.
var publishedStandardNames = []string{"d8-pod-security-baseline", "d8-pod-security-restricted"}

type securityPolicy struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels,omitempty"`
	} `json:"metadata"`
	Spec v1alpha1.SecurityPolicySpec `json:"spec"`
}

type ratifyReference struct {
	PublicKeys []string `json:"publicKeys"`
	Reference  string   `json:"reference"`
}
