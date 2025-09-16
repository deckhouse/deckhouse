// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type StatefulSetStorage struct {
	Kind           string
	APIVersion     string
	Name           string
	Namespace      string
	StorageRequest string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/upmeter/remove_old_sts",
	OnBeforeHelm: &go_hook.OrderedConfig{},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:         "sts",
			ApiVersion:   "apps/v1",
			Kind:         "StatefulSet",
			NameSelector: &types.NameSelector{MatchNames: []string{"upmeter"}},
			FilterFunc:   applyStsFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-upmeter"},
				},
			},
		},
	},
}, removeStsUpmeter)

func applyStsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sts appsv1.StatefulSet
	if err := sdk.FromUnstructured(obj, &sts); err != nil {
		return nil, err
	}

	if len(sts.Spec.VolumeClaimTemplates) == 0 {
		log.Debug("StatefulSet has no VolumeClaimTemplates", slog.String("namespace", sts.Namespace), slog.String("name", sts.Name))
		return nil, nil
	}

	quantity, ok := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		log.Debug("No storage resource request found in VolumeClaimTemplate", slog.String("namespace", sts.Namespace), slog.String("name", sts.Name))
		return nil, nil
	}

	return &StatefulSetStorage{
		Kind:           sts.Kind,
		APIVersion:     sts.APIVersion,
		Name:           sts.Name,
		Namespace:      sts.Namespace,
		StorageRequest: quantity.String(),
	}, nil
}

func removeStsUpmeter(_ context.Context, input *go_hook.HookInput) error {
	stsSnapshot := input.Snapshots.Get("sts")
	if len(stsSnapshot) > 0 {
		for sts, err := range sdkobjectpatch.SnapshotIter[StatefulSetStorage](stsSnapshot) {
			if err != nil {
				return fmt.Errorf("failed to iterate over snapshots: %w", err)
			}

			if sts.StorageRequest != "2Gi" {
				log.Debug("Deleting StatefulSet", slog.String("namespace", sts.Namespace), slog.String("name", sts.Name))
				input.PatchCollector.DeleteNonCascading(sts.APIVersion, sts.Kind, sts.Namespace, sts.Name)
			}
		}
	} else {
		log.Debug("StatefulSet not found")
	}
	return nil
}
