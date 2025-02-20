/*
Copyright 2025 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/user-authz/handle-dict-bindings",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "dictBindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":                    "deckhouse",
					"rbac.deckhouse.io/automated": "true",
					"rbac.deckhouse.io/dict":      "true",
				},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterManageBinding,
		},
		{
			Name:       "useBindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"deckhouse"},
					},
				},
			},
			FilterFunc: filterUseBinding,
		},
	},
}, ensureDictBindings)

func filterUseBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.RoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(binding.RoleRef.Name, "d8:use:role:") {
		return nil, nil
	}

	return &filteredUseBinding{
		Subjects: binding.Subjects,
	}, nil
}

func ensureDictBindings(input *go_hook.HookInput) error {
	subjects := make(map[string]rbacv1.Subject)
	for _, binding := range input.Snapshots["useBindings"] {
		if binding == nil {
			continue
		}

		parsed := binding.(*filteredUseBinding)
		if len(parsed.Subjects) == 0 {
			continue
		}

		for _, subject := range parsed.Subjects {
			subjects[stringBySubject(subject)] = subject
		}
	}

	for _, binding := range input.Snapshots["dictBindings"] {
		if binding == nil {
			continue
		}

		parsed := binding.(*filteredManageBinding)
		if parsed.Subjects == nil {
			continue
		}

		subjectString := stringBySubject(parsed.Subjects[0])

		if _, ok := subjects[subjectString]; !ok {
			input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", parsed.Name)
		}

		delete(subjects, subjectString)
	}

	for name, subject := range subjects {
		input.PatchCollector.Create(createDictBinding(name, subject), object_patch.IgnoreIfExists())
	}

	return nil
}

func createDictBinding(subjectString string, subject rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "d8:dict:",
			Annotations: map[string]string{
				"rbac.deckhouse.io/subject": subjectString,
			},
			Labels: map[string]string{
				"heritage":                    "deckhouse",
				"rbac.deckhouse.io/automated": "true",
				"rbac.deckhouse.io/dict":      "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "d8:use:dict",
		},
		Subjects: []rbacv1.Subject{subject},
	}
}

func stringBySubject(subject rbacv1.Subject) string {
	var str string
	if subject.Kind == "ServiceAccount" {
		subject.Kind = "sa"
	}
	if subject.Namespace == "" {
		str = fmt.Sprintf("%s:%s", subject.Kind, subject.Name)
	} else {
		str = fmt.Sprintf("%s:%s:%s", subject.Kind, subject.Namespace, subject.Name)
	}
	if len(str) > 55 {
		str = str[:55]
	}
	return strings.ToLower(str)
}
