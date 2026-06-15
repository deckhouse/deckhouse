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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	istioReservedUID        int64 = 1337
	istioProxyContainerName       = "istio-proxy"
	istioCanonicalNameLabel       = "service.istio.io/canonical-name"
	reservedUIDMetricsGroup       = "d8_istio_reserved_uid"
)

type podReservedUIDInfo struct {
	Namespace              string
	Pod                    string
	ImproperContainerNames []string
}

func applyReservedUIDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}

	var podRunAsUser *int64
	if pod.Spec.SecurityContext != nil {
		podRunAsUser = pod.Spec.SecurityContext.RunAsUser
	}

	var matchingContainers []string
	for _, container := range pod.Spec.Containers {
		if container.Name == istioProxyContainerName {
			continue
		}

		var effectiveRunAsUser *int64
		if container.SecurityContext != nil && container.SecurityContext.RunAsUser != nil {
			effectiveRunAsUser = container.SecurityContext.RunAsUser
		} else {
			effectiveRunAsUser = podRunAsUser
		}

		if effectiveRunAsUser != nil && *effectiveRunAsUser == istioReservedUID {
			matchingContainers = append(matchingContainers, container.Name)
		}
	}

	if len(matchingContainers) == 0 {
		return nil, nil
	}

	return podReservedUIDInfo{
		Namespace:              pod.Namespace,
		Pod:                    pod.Name,
		ImproperContainerNames: matchingContainers,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("monitoring"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyReservedUIDFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      istioCanonicalNameLabel,
						Operator: metav1.LabelSelectorOpExists,
					},
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"deckhouse"},
					},
				},
			},
		},
	},
}, handleReservedUIDMonitoring)

func handleReservedUIDMonitoring(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(reservedUIDMetricsGroup)

	for info, err := range sdkobjectpatch.SnapshotIter[podReservedUIDInfo](input.Snapshots.Get("pods")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over pod snapshots: %w", err)
		}

		for _, containerName := range info.ImproperContainerNames {
			input.MetricsCollector.Set(
				"d8_istio_pod_container_reserved_uid",
				1,
				map[string]string{
					"namespace": info.Namespace,
					"pod":       info.Pod,
					"container": containerName,
				},
				metrics.WithGroup(reservedUIDMetricsGroup),
			)
		}
	}

	return nil
}
