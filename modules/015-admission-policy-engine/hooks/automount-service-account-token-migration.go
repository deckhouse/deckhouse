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
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// migration/automount-service-account-token: "time.Now()"
const annotationName = "migrationAutomountServiceAccountTokenApplied"

func filterNS(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetAnnotations(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/admission-policy-engine/security_policies_migrations",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "security-policies",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "SecurityPolicy",
			FilterFunc: filterSP,
			// only snapshot update is needed
			ExecuteHookOnEvents:          go_hook.Bool(false),
			ExecuteHookOnSynchronization: go_hook.Bool(false),
		},
		{
			Name:       "ns",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: filterNS,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"d8-admission-policy-engine",
				},
			},
		},
	},
}, migrateSP)

func migrateSP(input *go_hook.HookInput) error {
	policyes := make([]*securityPolicy, 0)

	snap := input.Snapshots["security-policies"]
	ns := input.Snapshots["ns"]

	for _, sn := range snap {
		sp := sn.(*securityPolicy)
		sp.preprocesSecurityPolicy()
		policyes = append(policyes, sp)
	}

	if len(ns) == 0 {
		return nil
	}

	for _, v := range ns[0].(map[string]string) {
		if v == annotationName {
			return nil
		}
	}

	patchPolicy := map[string]interface{}{
		"spec": map[string]interface{}{
			"policies": map[string]bool{
				"automountServiceAccountToken": true,
			},
		},
	}
	for _, policy := range policyes {
		input.PatchCollector.MergePatch(patchPolicy, policy.APIVersion, policy.Kind, "", policy.Metadata.Name, object_patch.IgnoreMissingObject())
	}

	patchNamespace := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				annotationName: fmt.Sprint(time.Now().Unix()),
			},
		},
	}
	input.PatchCollector.MergePatch(patchNamespace, "v1", "Namespace", "", "d8-admission-policy-engine", object_patch.IgnoreMissingObject())

	return nil
}
