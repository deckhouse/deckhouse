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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const maxSpaceUtilization = 0.95

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/loki/calculate_storage_capacity",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pvcs",
			ApiVersion: "v1",
			Kind:       "PersistentVolumeClaim",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "loki",
				},
			},
			FilterFunc: persistentVolumeClaimFilter,
		},
	},
}, lokiDisk)

type PersistentVolumeClaim struct {
	Name            string
	RequestsStorage uint64
}

func persistentVolumeClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pvc = &corev1.PersistentVolumeClaim{}
	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	pvcSizeInBytes, ok := pvc.Spec.Resources.Requests.Storage().AsInt64()
	if !ok {
		return nil, fmt.Errorf("cannot get .Spec.Resources.Requests from PersistentVolumeClaim %s", pvc.Name)
	}

	return PersistentVolumeClaim{
		Name:            pvc.Name,
		RequestsStorage: uint64(pvcSizeInBytes),
	}, nil
}

func lokiDisk(input *go_hook.HookInput) error {
	var pvcSize, cleanupThreshold uint64

	defaultDiskSize := uint64(input.ConfigValues.Get("loki.diskSizeGigabytes").Int() << 30)
	ingestionRate := input.ConfigValues.Get("loki.lokiConfig.ingestionRateMB").Float()

	for _, obj := range input.Snapshots["pvcs"] {
		pvc := obj.(PersistentVolumeClaim)

		if !strings.HasSuffix(pvc.Name, "-0") {
			continue
		}

		pvcSize = pvc.RequestsStorage
		break
	}

	if pvcSize == 0 {
		pvcSize = defaultDiskSize
	}

	cleanupThreshold = pvcSize - uint64(ingestionRate*1024*1024)*60*2 // Reserve twice size of WALs for a minute (checkpoint interval)

	// do not exceed 95% of the PVC size
	if float64(cleanupThreshold) > float64(pvcSize)*maxSpaceUtilization {
		cleanupThreshold = uint64(float64(pvcSize) * maxSpaceUtilization)
	}

	input.Values.Set("loki.internal.pvcSize", pvcSize)
	input.Values.Set("loki.internal.cleanupThreshold", cleanupThreshold)

	return nil
}
