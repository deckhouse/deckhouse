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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/031-local-path-provisioner/hooks/internal/v1alpha1"
)

type StorageClass struct {
	Name          string
	ReclaimPolicy string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/local-path-provisioner",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_storageclasses",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"module": "local-path-provisioner"},
			},
			FilterFunc: applyModuleStorageClassesFilter,
		},
		{
			Name:       "module_crds",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "LocalPathProvisioner",
			FilterFunc: applyModuleCRDFilter,
		},
	},
}, storageClasses)

func applyModuleStorageClassesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sc = &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return StorageClass{Name: sc.Name, ReclaimPolicy: string(*sc.ReclaimPolicy)}, nil
}

func applyModuleCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	lpp := new(v1alpha1.LocalPathProvisioner)
	err := sdk.FromUnstructured(obj, lpp)
	if err != nil {
		return nil, err
	}

	return StorageClass{
		Name:          lpp.Name,
		ReclaimPolicy: lpp.Spec.ReclaimPolicy,
	}, nil
}

func storageClasses(input *go_hook.HookInput) error {
	if len(input.Snapshots["module_storageclasses"]) == 0 || len(input.Snapshots["module_crds"]) == 0 {
		return nil
	}

	existedStorageClasses := make([]StorageClass, 0, len(input.Snapshots["module_storageclasses"]))

	for _, snapshot := range input.Snapshots["module_storageclasses"] {
		sc := snapshot.(StorageClass)
		existedStorageClasses = append(existedStorageClasses, sc)
	}

	for _, snapshot := range input.Snapshots["module_crds"] {
		crd := snapshot.(StorageClass)
		for _, storageClass := range existedStorageClasses {
			if storageClass.Name == crd.Name {
				if storageClass.ReclaimPolicy != crd.ReclaimPolicy {
					input.Logger.Infof("Deleting storageclass/%s because its parameters has been changed", storageClass.Name)
					input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", storageClass.Name)
				}
				break
			}
		}
	}
	return nil
}
