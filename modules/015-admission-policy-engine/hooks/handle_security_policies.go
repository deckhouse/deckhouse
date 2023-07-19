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
	"strings"
	"time"

	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
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
}, dependency.WithExternalDependencies(handleSP))

func handleSP(input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	result := make([]*securityPolicy, 0)

	snap := input.Snapshots["security-policies"]

	for _, sn := range snap {
		sp := sn.(*securityPolicy)
		sp.preprocesSecurityPolicy()
		result = append(result, sp)
		// annotate an sp object as noticed
		if err := annotateWithNoticed(sp, kubeClient); err != nil {
			return fmt.Errorf("cannot annotate security policy: %v", err)
		}
	}

	data, _ := json.Marshal(result)

	input.Values.Set("admissionPolicyEngine.internal.securityPolicies", json.RawMessage(data))

	return nil
}

// set deckhouse.io/admission-policy-engine-hook-noticed and deckhouse.io/admission-policy-engine-hook-synced annotations
func annotateWithNoticed(sp *securityPolicy, kubeClient k8s.Client) error {
	spInterface := kubeClient.Dynamic().Resource(schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "securitypolicies"}).Namespace("")
	spObj, err := spInterface.Get(context.TODO(), sp.Metadata.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	spBytes, err := json.Marshal(sp)
	if err != nil {
		return err
	}
	checkSum := utils_checksum.CalculateChecksum(string(spBytes))

	processedAnnotation, found, err := unstructured.NestedString(spObj.Object, "metadata", "annotations", "deckhouse.io/admission-policy-engine-hook-processed")
	if err != nil {
		return err
	}

	if !found || !checksumEqualsAnnotation(checkSum, processedAnnotation) {
		if err := unstructured.SetNestedField(spObj.Object, "False", "metadata", "annotations", "deckhouse.io/admission-policy-engine-hook-synced"); err != nil {
			return err
		}
	}

	if err := unstructured.SetNestedField(spObj.Object, fmt.Sprintf("%s/%s", time.Now().Format(time.RFC3339), checkSum), "metadata", "annotations", "deckhouse.io/admission-policy-engine-hook-noticed"); err != nil {
		return err
	}
	_, err = spInterface.Update(context.TODO(), spObj, metav1.UpdateOptions{})
	return err
}

func checksumEqualsAnnotation(checkSum, annotation string) bool {
	splitAnnotation := strings.Split(annotation, "/")
	if len(splitAnnotation) != 2 {
		return false
	}
	return splitAnnotation[1] == checkSum
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
}

type securityPolicy struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec v1alpha1.SecurityPolicySpec `json:"spec"`
}
