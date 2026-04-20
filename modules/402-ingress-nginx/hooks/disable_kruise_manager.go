/*
Copyright 2026 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

const (
	kruiseManagerDeploymentName = "kruise-controller-manager"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/ingress-nginx/disable_kruise_manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kruise_manager",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NameSelector: &types.NameSelector{
				MatchNames: []string{kruiseManagerDeploymentName},
			},
			NamespaceSelector: internal.NsSelector(),
			FilterFunc:        applyKruiseManagerDeploymentFilter,
		},
	},
}, disableKruiseManager)

func applyKruiseManagerDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var deployment appsv1.Deployment
	if err := sdk.FromUnstructured(obj, &deployment); err != nil {
		return nil, err
	}

	var replicas int32 = 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	return replicas, nil
}

func disableKruiseManager(_ context.Context, input *go_hook.HookInput) error {
	if input.Values.Get("ingressNginx.internal.legacyKruiseManagementEnabled").Bool() {
		return nil
	}

	snapshot := input.Snapshots.Get("kruise_manager")
	if len(snapshot) == 0 {
		return nil
	}

	var replicas int32
	if err := snapshot[0].UnmarshalTo(&replicas); err != nil {
		return err
	}

	if replicas == 0 {
		return nil
	}

	patch := map[string]any{
		"spec": map[string]any{
			"replicas": 0,
		},
	}

	input.PatchCollector.PatchWithMerge(
		patch,
		"apps/v1",
		"Deployment",
		internal.Namespace,
		kruiseManagerDeploymentName,
	)

	return nil
}
