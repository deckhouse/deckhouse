/*
Copyright 2025 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const maxSpaceUtilization = 0.92

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
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   persistentVolumeClaimFilter,
		},
		{
			Name:       "sts",
			ApiVersion: "apps/v1",
			Kind:       "StatefulSet",
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
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   statefulSetFilter,
		},
	},
}, lokiDisk)

type PersistentVolumeClaim struct {
	Name            string
	RequestsStorage uint64
}

type StatefulSet struct {
	Name       string
	VolumeSize uint64
}

func statefulSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sts := &appsv1.StatefulSet{}
	err := sdk.FromUnstructured(obj, sts)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %w", err)
	}

	volumeSize := uint64(0)
	for _, volume := range sts.Spec.VolumeClaimTemplates {
		if volume.Name == "storage" {
			size, ok := volume.Spec.Resources.Requests.Storage().AsInt64()
			if !ok {
				return nil, fmt.Errorf("cannot get .Spec.Resources.Requests from sts/%s, VolumeClaimTemplate %s", sts.Name, volume.Name)
			}
			volumeSize = uint64(size)
			break
		}
	}

	if volumeSize == 0 {
		for _, volume := range sts.Spec.Template.Spec.Volumes {
			if volume.Name == "storage" && volume.EmptyDir != nil {
				size, ok := volume.EmptyDir.SizeLimit.AsInt64()
				if !ok {
					return nil, fmt.Errorf("cannot get .SizeLimit from sts/%s, EmptyDir %s", sts.Name, volume.Name)
				}
				volumeSize = uint64(size)
				break
			}
		}
	}

	return StatefulSet{
		Name:       sts.Name,
		VolumeSize: volumeSize,
	}, nil
}

func persistentVolumeClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pvc := &corev1.PersistentVolumeClaim{}
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

func lokiDisk(_ context.Context, input *go_hook.HookInput) error {
	var stsStorageSize, pvcSize, cleanupThreshold uint64

	defaultDiskSize := uint64(input.Values.Get("loki.diskSizeGigabytes").Int() << 30)
	ingestionRate := input.Values.Get("loki.lokiConfig.ingestionRateMB").Float()

	for pvc, err := range sdkobjectpatch.SnapshotIter[PersistentVolumeClaim](input.Snapshots.Get("pvcs")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'pvcs' snapshots: %w", err)
		}

		if !strings.HasSuffix(pvc.Name, "-0") {
			continue
		}

		pvcSize = pvc.RequestsStorage
		break
	}

	for sts, err := range sdkobjectpatch.SnapshotIter[StatefulSet](input.Snapshots.Get("sts")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'sts' snapshots: %w", err)
		}

		if sts.Name != "loki" {
			continue
		}

		stsStorageSize = sts.VolumeSize
		break
	}

	if pvcSize == 0 {
		pvcSize = defaultDiskSize
	}

	if stsStorageSize == 0 {
		stsStorageSize = pvcSize
	}

	reservedByWALs := uint64(ingestionRate*1024*1024) * 60 * 2

	if pvcSize <= reservedByWALs {
		return fmt.Errorf("PVC size is less or equal than reserved space for WALs. Too high ingestionRateMB (%f) or too small PVC size (%d)", ingestionRate, pvcSize)
	}

	cleanupThreshold = pvcSize - reservedByWALs

	// do not exceed 92% of the PVC size
	if float64(cleanupThreshold) > float64(pvcSize)*maxSpaceUtilization {
		cleanupThreshold = uint64(float64(pvcSize) * maxSpaceUtilization)
	}

	input.Values.Set("loki.internal.pvcSize", pvcSize)
	input.Values.Set("loki.internal.stsStorageSize", stsStorageSize)
	input.Values.Set("loki.internal.cleanupThreshold", cleanupThreshold)

	return nil
}
