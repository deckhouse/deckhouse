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
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	defaultDiskSizeGB           = 40
	defaultDiskRetentionPercent = 80
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/prometheus/prometheus_disk",
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
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "prometheus",
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
	RetentionPercent int
}

func prometheusDisk(input *go_hook.HookInput) error {
	var main storage
	var longterm storage

	main.VolumeSizeGiB = defaultDiskSizeGB
	main.RetentionPercent = defaultDiskRetentionPercent
	longterm.VolumeSizeGiB = defaultDiskSizeGB
	longterm.RetentionPercent = defaultDiskRetentionPercent

	for _, obj := range input.Snapshots["pvcs"] {
		pvc := obj.(PersistentVolumeClaim)
		switch pvc.PrometheusName {
		case "main":
			main.VolumeSizeGiB = pvc.RequestsStorage
		case "longterm":
			longterm.VolumeSizeGiB = pvc.RequestsStorage
		default:
			continue
		}
	}

	if input.ConfigValues.Exists("prometheus.diskRetentionPercent") {
		main.RetentionPercent = int(input.ConfigValues.Get("prometheus.retentionPercent").Int())
	}

	if input.ConfigValues.Exists("prometheus.diskRetentionPercent") {
		longterm.RetentionPercent = int(input.ConfigValues.Get("prometheus.longtermRetentionPercent").Int())
	}

	if main.RetentionPercent == 0 {
		main.RetentionPercent = defaultDiskRetentionPercent
	}

	if longterm.RetentionPercent == 0 {
		longterm.RetentionPercent = defaultDiskRetentionPercent
	}

	main.RetentionSizeGiB = int(math.Round(float64(main.VolumeSizeGiB) * float64(main.RetentionPercent) / 100.0))
	longterm.RetentionSizeGiB = int(math.Round(float64(longterm.VolumeSizeGiB) * float64(longterm.RetentionPercent) / 100.0))

	input.Values.Set("prometheus.internal.prometheusMain.diskSizeGigabytes", main.VolumeSizeGiB)
	input.Values.Set("prometheus.internal.prometheusMain.retentionGigabytes", main.RetentionSizeGiB)

	input.Values.Set("prometheus.internal.prometheusLongterm.diskSizeGigabytes", longterm.VolumeSizeGiB)
	input.Values.Set("prometheus.internal.prometheusLongterm.retentionGigabytes", longterm.RetentionSizeGiB)

	return nil
}
