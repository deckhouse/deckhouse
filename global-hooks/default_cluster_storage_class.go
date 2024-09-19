// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeAll: &go_hook.OrderedConfig{Order: 25},
	// TODO: протестить conversion для global.storageClass -> global.modules.storageClass
	// TODO: убрать логику internal.defaultStorageClass из helm
	// TODO: проверить, что в дефолтном кластере (только что созданном) storageClass-ы НЕ МЕНЯЮТСЯ (если не задан `global.defaultStorageClass`)

	// Kubernetes: []go_hook.KubernetesConfig{
	// 	{
	// 		Name:       "default_cluster_sc",
	// 		ApiVersion: "v1",
	// 		Kind:       "ConfigMap",
	// 		NameSelector: &types.NameSelector{
	// 			MatchNames: []string{"d8-default-cluster-storage-class"},
	// 		},
	// 		NamespaceSelector: &types.NamespaceSelector{
	// 			NameSelector: &types.NameSelector{
	// 				MatchNames: []string{d8Namespace},
	// 			},
	// 		},
	// 		FilterFunc: applyDefaultClusterStorageClassCmFilter,
	// 	},
	// },
}, dependency.WithExternalDependencies(setupDefaultStorageClass))

func applyDefaultClusterStorageClassCmFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// slightly modified code from go_lib/filter/extract.go/KeyFromConfigMap
	const key = "default-cluster-storage-class"

	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		// if no configmap - no problem
		return "", err
	}

	val, ok := cm.Data[key]
	if !ok {
		// if no key in configmap - no problem
		return "", nil
	}

	return val, nil
}

func setupDefaultStorageClass(input *go_hook.HookInput, dc dependency.Container) error {
	defaultClusterStorageClassSnap := input.Snapshots["default_cluster_sc"]

	if len(defaultClusterStorageClassSnap) == 0 || defaultClusterStorageClassSnap[0] == "" {
		input.LogEntry.Infoln("Default cluster storage class configmap not found or empty. Skipping")
		return nil
	}

	defaultClusterStorageClass := defaultClusterStorageClassSnap[0]

	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	storageClasses, err := client.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		input.LogEntry.Warnf("Error getting storage classes: %s", err)
		return nil
	}

	for _, sc := range storageClasses.Items {
		if sc.GetName() == defaultClusterStorageClass {
			// it's that storage class which we want
			if !isMarkedDefault(&sc) {
				// we must add default-annotation to this StorageClass because it's not annotated as default
				input.LogEntry.Warnf("Add default annotation to storage class %q (it specified in `global.defaultClusterStorageClass`)", sc.GetName())

				patch := map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"storageclass.kubernetes.io/is-default-class": "true",
						},
					},
				}

				input.PatchCollector.MergePatch(patch, "storage.k8s.io/v1", "StorageClass", "", sc.GetName())
			}
		} else {
			if isMarkedDefault(&sc) {
				// we must remove default-annotation from this StorageClass because only one StorageClass (which name in defaultClusterStorageClass) can be default
				input.LogEntry.Warnf("Remove default annotations from storage class %q", sc.GetName())

				patch := map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"storageclass.beta.kubernetes.io/is-default-class": nil,
							"storageclass.kubernetes.io/is-default-class":      nil,
						},
					},
				}

				input.PatchCollector.MergePatch(patch, "storage.k8s.io/v1", "StorageClass", "", sc.GetName())
			}
		}
	}

	return nil
}

func isMarkedDefault(sc *storage.StorageClass) bool {
	annotations := sc.GetAnnotations()

	annotToCheck := []string{
		"storageclass.beta.kubernetes.io/is-default-class",
		"storageclass.kubernetes.io/is-default-class",
	}

	isDefault := false
	for _, annot := range annotToCheck {
		if v, ok := annotations[annot]; ok && strings.ToLower(v) == "true" {
			isDefault = true
			break
		}
	}

	return isDefault
}
