/*
Copyright 2023 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

const (
	istioSystemNs = "d8-istio"
)

type removeObject struct {
	APIVersion              string
	Kind                    string
	Namespace               string
	Name                    string
	DeletionTimestampExists bool
	FinalizersExists        bool
}

var (
	deleteFinalizersPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": nil,
		},
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "clwNamespace",
			ApiVersion:                   "v1",
			Kind:                         "Namespace",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveNsFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{istioSystemNs},
			},
		},
		{
			Name:                         "clwClusterRole",
			ApiVersion:                   "rbac.authorization.k8s.io/v1",
			Kind:                         "ClusterRole",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"install.operator.istio.io/owning-resource-namespace": "d8-istio",
				},
			},
		},
		{
			Name:                         "clwClusterRoleBinding",
			ApiVersion:                   "rbac.authorization.k8s.io/v1",
			Kind:                         "ClusterRoleBinding",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"install.operator.istio.io/owning-resource-namespace": "d8-istio",
				},
			},
		},
		{
			Name:                         "clwMutatingWebhookConfiguration",
			ApiVersion:                   "admissionregistration.k8s.io/v1",
			Kind:                         "MutatingWebhookConfiguration",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"install.operator.istio.io/owning-resource-namespace": "d8-istio",
				},
			},
		},
		{
			Name:                         "clwValidatingWebhookConfiguration",
			ApiVersion:                   "admissionregistration.k8s.io/v1",
			Kind:                         "ValidatingWebhookConfiguration",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"install.operator.istio.io/owning-resource-namespace": "d8-istio",
				},
			},
		},
		{
			Name:                         "nsdServiceAccount",
			ApiVersion:                   "v1",
			Kind:                         "ServiceAccount",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdRole",
			ApiVersion:                   "rbac.authorization.k8s.io/v1",
			Kind:                         "Role",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdRoleBinding",
			ApiVersion:                   "rbac.authorization.k8s.io/v1",
			Kind:                         "RoleBinding",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdIstioOperator",
			ApiVersion:                   "install.istio.io/v1alpha1",
			Kind:                         "IstioOperator",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdEnvoyFilter",
			ApiVersion:                   "networking.istio.io/v1alpha3",
			Kind:                         "EnvoyFilter",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdDeployment",
			ApiVersion:                   "apps/v1",
			Kind:                         "Deployment",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdReplicaSet",
			ApiVersion:                   "apps/v1",
			Kind:                         "ReplicaSet",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdPod",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdConfigMap",
			ApiVersion:                   "v1",
			Kind:                         "ConfigMap",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdPodDisruptionBudget",
			ApiVersion:                   "policy/v1",
			Kind:                         "PodDisruptionBudget",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
		{
			Name:                         "nsdService",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyRemoveObjectFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{istioSystemNs},
				},
			},
		},
	},
}, purgeOrphanResources)

func applyRemoveNsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	_, deletionTimestampExists := obj.GetAnnotations()["deletionTimestamp"]

	var RemoveNsInfo = removeObject{
		APIVersion:              obj.GetAPIVersion(),
		Kind:                    obj.GetKind(),
		Name:                    obj.GetName(),
		DeletionTimestampExists: deletionTimestampExists,
	}

	return RemoveNsInfo, nil
}

func applyRemoveObjectFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	_, deletionTimestampExists := obj.GetAnnotations()["deletionTimestamp"]
	// _, finalizersExists := obj.GetFinalizers()
	var finalizersExists bool
	if len(obj.GetFinalizers()) == 0 {
		finalizersExists = false
	} else {
		finalizersExists = true
	}

	var RemoveObjectInfo = removeObject{
		APIVersion:              obj.GetAPIVersion(),
		Kind:                    obj.GetKind(),
		Namespace:               obj.GetNamespace(),
		Name:                    obj.GetName(),
		FinalizersExists:        finalizersExists,
		DeletionTimestampExists: deletionTimestampExists,
	}

	return RemoveObjectInfo, nil
}

func purgeOrphanResources(input *go_hook.HookInput) error {
	objects := make([]go_hook.FilterResult, 0)
	objects = append(objects, input.Snapshots["clwClusterRole"]...)
	objects = append(objects, input.Snapshots["clwClusterRoleBinding"]...)
	objects = append(objects, input.Snapshots["clwMutatingWebhookConfiguration"]...)
	objects = append(objects, input.Snapshots["clwValidatingWebhookConfiguration"]...)
	objects = append(objects, input.Snapshots["nsdServiceAccount"]...)
	objects = append(objects, input.Snapshots["nsdRole"]...)
	objects = append(objects, input.Snapshots["nsdRoleBinding"]...)
	objects = append(objects, input.Snapshots["nsdIstioOperator"]...)
	objects = append(objects, input.Snapshots["nsdEnvoyFilter"]...)
	objects = append(objects, input.Snapshots["nsdDeployment"]...)
	// objects = append(objects, input.Snapshots["nsdReplicaSet"]...)
	// objects = append(objects, input.Snapshots["nsdPod"]...)
	objects = append(objects, input.Snapshots["nsdConfigMap"]...)
	objects = append(objects, input.Snapshots["nsdPodDisruptionBudget"]...)
	objects = append(objects, input.Snapshots["nsdService"]...)
	objects = append(objects, input.Snapshots["nsdConfigMap"]...)

	for _, objRaw := range objects {
		obj := objRaw.(removeObject)
		if obj.FinalizersExists {
			input.PatchCollector.MergePatch(deleteFinalizersPatch, obj.APIVersion, obj.Kind, obj.Namespace, obj.Name)
		}
		input.PatchCollector.Delete(obj.APIVersion, obj.Kind, obj.Namespace, obj.Name, object_patch.InForeground())
	}

	if len(input.Snapshots["clwNamespace"]) > 0 {
		ns := input.Snapshots["clwNamespace"][0].(removeObject)
		if !ns.DeletionTimestampExists {
			input.PatchCollector.Delete(ns.APIVersion, ns.Kind, "", ns.Name, object_patch.InForeground())
		}
	}

	return nil
}
