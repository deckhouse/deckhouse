/*
Copyright 2021 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ceph-csi",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name: "main",
			Crontab: "*/15 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "scs",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "Storageclass",
			FilterFunc: applyStorageclassFilter,
		},
		{
			Name:       "pvcs",
			ApiVersion: "v1",
			Kind:       "PersistentVolumeClaim",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "prometheus",
				},
			},
			FilterFunc: applyPersistentVolumeClaimFilter,
		},
		{
			Name:       "pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "prometheus",
				},
			},
			FilterFunc: applyPodFilter,
		},
	},
}, dependency.WithExternalDependencies(prometheusDisk))

type StorageClassFilter struct {
	Name                 string
	AllowVolumeExpansion bool
}

func applyStorageclassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sc = &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return StorageClassFilter{
		Name:                 sc.Name,
		AllowVolumeExpansion: *sc.AllowVolumeExpansion,
	}, nil
}

type PersistentVolumeClaimFilter struct {
	Name            string
	RequestsStorage string
}

func applyPersistentVolumeClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pvc = &corev1.PersistentVolumeClaim{}
	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return PersistentVolumeClaimFilter{
		Name:            pvc.Name,
		RequestsStorage: pvc.Spec.Resources.Requests.Storage().String(),
	}, nil
}

type PodFilter struct {
	Name string
}

func applyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod = &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return PodFilter{
		Name: pod.Name,
	}, nil
}

func prometheusDisk(input *go_hook.HookInput, dc dependency.Container) error {

	//kubeClient, err := dc.GetK8sClient()
	//if err != nil {
	//	return err
	//}

	//crs := input.Snapshots["crs"]

	//input.Values.Set("cephCsi.internal.csiConfig", csiConfig)

	return nil
}
