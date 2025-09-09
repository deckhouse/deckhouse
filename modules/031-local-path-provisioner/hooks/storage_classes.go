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
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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

func storageClasses(_ context.Context, input *go_hook.HookInput) error {
	moduleStorageClasses, err := sdkobjectpatch.UnmarshalToStruct[StorageClass](input.Snapshots, "module_storageclasses")
	if err != nil {
		return fmt.Errorf("failed to unmarshal module_storageclasses snapshot: %w", err)
	}
	moduleCRDs, err := sdkobjectpatch.UnmarshalToStruct[StorageClass](input.Snapshots, "module_crds")
	if err != nil {
		return fmt.Errorf("failed to unmarshal module_crds snapshot: %w", err)
	}

	if len(moduleStorageClasses) == 0 || len(moduleCRDs) == 0 {
		return nil
	}

	existedStorageClasses := moduleStorageClasses

	for _, crd := range moduleCRDs {
		for _, storageClass := range existedStorageClasses {
			if storageClass.Name == crd.Name {
				if storageClass.ReclaimPolicy != crd.ReclaimPolicy {
					input.Logger.Info("Deleting storageclass because its parameters have been changed", slog.String("storage_class", storageClass.Name))
					input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", storageClass.Name)
				}
				break
			}
		}
	}

	return nil
}
