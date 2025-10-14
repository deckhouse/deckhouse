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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storage "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type StorageClassDup struct {
	Name      string
	IsDefault bool
}

func filterStorageClassDup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageclass := new(storage.StorageClass)
	err := sdk.FromUnstructured(obj, storageclass)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes obj to Storageclass: %v", err)
	}

	s := StorageClassDup{}
	s.Name = storageclass.ObjectMeta.Name
	for k, v := range storageclass.ObjectMeta.Annotations {
		if v == "true" && (k == "storageclass.beta.kubernetes.io/is-default-class" || k == "storageclass.kubernetes.io/is-default-class") {
			s.IsDefault = true
			break
		}
	}

	return s, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "storageclasses",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "Storageclass",
			FilterFunc: filterStorageClassDup,
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpNotIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
		},
	},
}, delectStorageClassDuplicate)

func delectStorageClassDuplicate(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("")

	storageclasses := input.Snapshots.Get("storageclasses")

	var defaultStorageclasses int64
	for sc, err := range sdkobjectpatch.SnapshotIter[StorageClassDup](storageclasses) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'storageclasses' snapshots: %w", err)
		}

		if sc.IsDefault {
			defaultStorageclasses++
		}
	}

	if defaultStorageclasses > 1 {
		for sc, err := range sdkobjectpatch.SnapshotIter[StorageClassDup](storageclasses) {
			if err != nil {
				return fmt.Errorf("failed to iterate over 'storageclasses' snapshots: %w", err)
			}

			if sc.IsDefault {
				input.MetricsCollector.Set(
					"storage_class_default_duplicate",
					1.0,
					map[string]string{
						"name": sc.Name,
					},
				)
			}
		}
	}

	return nil
}
