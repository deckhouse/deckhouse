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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses"
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
		// set observed status
		input.PatchCollector.Filter(set_cr_statuses.SetObservedStatus(sn, filterSP), "deckhouse.io/v1alpha1", "securitypolicy", "", sp.Metadata.Name, object_patch.WithSubresource("/status"), object_patch.IgnoreHookError())
		sp.preprocesSecurityPolicy()
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

func hasItem(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func (sp *securityPolicy) preprocesSecurityPolicy() {
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
		sp.Spec.Policies.AutomountServiceAccountToken = pointer.Bool(true)
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
