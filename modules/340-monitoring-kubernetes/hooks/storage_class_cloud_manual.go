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

	"github.com/deckhouse/deckhouse/go_lib/set"
)

type CloudStorageClass struct {
	Name        string
	IsCloud     bool
	Provisioner string
}

var cloudProvisioners = set.New("ebs.csi.aws.com", "disk.csi.azure.com", "pd.csi.storage.gke.io", "cinder.csi.openstack.org", "vsphere.csi.vmware.com", "yandex.csi.flant.com")

func filterStorageClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageclass := new(storage.StorageClass)
	err := sdk.FromUnstructured(obj, storageclass)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes obj to Storageclass: %v", err)
	}

	return CloudStorageClass{
		Name:        storageclass.ObjectMeta.Name,
		Provisioner: storageclass.Provisioner,
		IsCloud:     cloudProvisioners.Has(storageclass.Provisioner),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "storageclasses",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "Storageclass",
			FilterFunc: filterStorageClass,
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
}, delectStorageClassCloudManual)

func delectStorageClassCloudManual(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("")

	storageclasses := input.Snapshots.Get("storageclasses")

	for sc, err := range sdkobjectpatch.SnapshotIter[CloudStorageClass](storageclasses) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'storageclasses' snapshots: %w", err)
		}

		if sc.IsCloud && (sc.Name != "vsphere-main" || sc.Provisioner != "vsphere.csi.vmware.com") {
			input.MetricsCollector.Set(
				"storage_class_cloud_manual",
				1.0,
				map[string]string{
					"name": sc.Name,
				},
			)
		}
	}

	return nil
}
