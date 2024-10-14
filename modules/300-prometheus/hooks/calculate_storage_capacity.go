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
	"math"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const defaultDiskSizeGiB = 40
const retentionPercent = 85
const maxFreeSpaceGiB = 50

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/prometheus/calculate_storage_capacity",
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
			FilterFunc: persistentVolumeClaimFilter,
		},
	},
}, prometheusDisk)

type PersistentVolumeClaim struct {
	Name            string
	RequestsStorage int
	PrometheusName  string
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
		RequestsStorage: int(pvcSizeInBytes / 1024 / 1024 / 1024),
		PrometheusName:  pvc.Labels["prometheus"],
	}, nil
}

type storage struct {
	VolumeSizeGiB    int
	RetentionSizeGiB int
}

func prometheusDisk(input *go_hook.HookInput) error {
	var main storage
	var longterm storage

	highAvailability := false

	if input.Values.Exists("global.highAvailability") {
		highAvailability = input.Values.Get("global.highAvailability").Bool()
	}
	if input.Values.Exists("prometheus.highAvailability") {
		highAvailability = input.Values.Get("prometheus.highAvailability").Bool()
	}

	for _, obj := range input.Snapshots["pvcs"] {
		pvc := obj.(PersistentVolumeClaim)

		if !highAvailability && !strings.HasSuffix(pvc.Name, "-0") {
			continue
		}

		switch pvc.PrometheusName {
		case "main":
			if main.VolumeSizeGiB < pvc.RequestsStorage {
				main.VolumeSizeGiB = pvc.RequestsStorage
			}
		case "longterm":
			if longterm.VolumeSizeGiB < pvc.RequestsStorage {
				longterm.VolumeSizeGiB = pvc.RequestsStorage
			}
		default:
			continue
		}
	}

	if main.VolumeSizeGiB == 0 {
		main.VolumeSizeGiB = defaultDiskSizeGiB
	}

	if longterm.VolumeSizeGiB == 0 {
		longterm.VolumeSizeGiB = defaultDiskSizeGiB
	}

	main.RetentionSizeGiB = int(math.Round(float64(main.VolumeSizeGiB) * float64(retentionPercent) / 100.0))
	if (main.VolumeSizeGiB - main.RetentionSizeGiB) > maxFreeSpaceGiB {
		main.RetentionSizeGiB = main.VolumeSizeGiB - maxFreeSpaceGiB
	}

	longterm.RetentionSizeGiB = int(math.Round(float64(longterm.VolumeSizeGiB) * float64(retentionPercent) / 100.0))
	if (longterm.VolumeSizeGiB - longterm.RetentionSizeGiB) > maxFreeSpaceGiB {
		longterm.RetentionSizeGiB = longterm.VolumeSizeGiB - maxFreeSpaceGiB
	}

	input.Values.Set("prometheus.internal.prometheusMain.diskSizeGigabytes", main.VolumeSizeGiB)
	input.Values.Set("prometheus.internal.prometheusMain.retentionGigabytes", main.RetentionSizeGiB)

	input.Values.Set("prometheus.internal.prometheusLongterm.diskSizeGigabytes", longterm.VolumeSizeGiB)
	input.Values.Set("prometheus.internal.prometheusLongterm.retentionGigabytes", longterm.RetentionSizeGiB)

	// remove deprecated parameters from configmap to further remove them from the openapi spec

	if input.ConfigValues.Exists("prometheus.mainMaxDiskSizeGigabytes") {
		input.ConfigValues.Remove("prometheus.mainMaxDiskSizeGigabytes")
	}

	if input.ConfigValues.Exists("prometheus.longtermMaxDiskSizeGigabytes") {
		input.ConfigValues.Remove("prometheus.longtermMaxDiskSizeGigabytes")
	}

	return nil
}
