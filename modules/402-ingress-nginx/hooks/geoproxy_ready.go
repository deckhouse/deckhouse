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

	gohook "github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

var _ = sdk.RegisterFunc(&gohook.HookConfig{
	Kubernetes: []gohook.KubernetesConfig{
		{
			Name:                         "geoproxy",
			ApiVersion:                   "apps/v1",
			Kind:                         "StatefulSet",
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			WaitForSynchronization:       ptr.To(true),
			NamespaceSelector:            internal.NsSelector(),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"geoproxy"},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "geoproxy",
				},
			},
			FilterFunc: applyGeoProxy,
		},
	},
}, handleMaxMindSettings)

func applyGeoProxy(obj *unstructured.Unstructured) (gohook.FilterResult, error) {
	ss := &appsv1.StatefulSet{}
	err := sdk.FromUnstructured(obj, ss)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	desiredReplicas := ptr.Deref(ss.Spec.Replicas, 0)
	ready := desiredReplicas > 0 &&
		ss.Status.ObservedGeneration >= ss.Generation &&
		ss.Status.ReadyReplicas == desiredReplicas &&
		ss.Status.UpdatedReplicas == desiredReplicas

	return ready, nil
}

func handleMaxMindSettings(_ context.Context, input *gohook.HookInput) error {
	ready := false

	for isReady, err := range sdkobjectpatch.SnapshotIter[bool](input.Snapshots.Get("geoproxy")) {
		if err != nil {
			return fmt.Errorf("failed to parse geoproxy snapshot: %w", err)
		}

		if isReady {
			ready = true
			break
		}
	}

	// Once geoproxy is ready, keep the flag true to avoid flapping.
	existingReady := input.Values.Get("ingressNginx.internal.geoproxyReady").Bool()
	input.Values.Set("ingressNginx.internal.geoproxyReady", ready || existingReady)

	return nil
}
