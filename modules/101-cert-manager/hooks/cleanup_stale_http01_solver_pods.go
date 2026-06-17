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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

const solverPodsSnapshot = "solverPods"

type solverPod struct {
	Namespace    string
	Name         string
	Phase        corev1.PodPhase
	BeingDeleted bool
}

func applySolverPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	if err := sdk.FromUnstructured(obj, &pod); err != nil {
		return nil, err
	}

	return solverPod{
		Namespace:    pod.GetNamespace(),
		Name:         pod.GetName(),
		Phase:        pod.Status.Phase,
		BeingDeleted: pod.GetDeletionTimestamp() != nil,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("cleanup-stale-http01-solver-pods"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       solverPodsSnapshot,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"acme.cert-manager.io/http01-solver": "true",
				},
			},
			FilterFunc: applySolverPodFilter,
		},
	},
}, cleanupStaleHTTP01SolverPods)

func cleanupStaleHTTP01SolverPods(_ context.Context, input *go_hook.HookInput) error {
	for _, podSnap := range input.Snapshots.Get(solverPodsSnapshot) {
		var pod solverPod
		if err := podSnap.UnmarshalTo(&pod); err != nil {
			return err
		}

		if pod.BeingDeleted {
			continue
		}

		if pod.Phase != corev1.PodSucceeded && pod.Phase != corev1.PodFailed && pod.Phase != corev1.PodUnknown {
			continue
		}

		input.PatchCollector.DeleteInBackground("v1", "Pod", pod.Namespace, pod.Name)
	}

	return nil
}
