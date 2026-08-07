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
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	proxyNamespace    = "d8-cloud-instance-manager"
	proxyAppLabel     = "registry-packages-proxy"
	proxyPodsSnapshot = "proxy_pods"

	// proxyAddressesValuesPath holds the addresses clients connect to. The
	// certificate hook reads them to build its SAN list.
	proxyAddressesValuesPath = "registryPackagesProxy.internal.proxyAddresses"
)

// Collect the addresses clients connect to. The proxy pods run in the host
// network, so a pod IP is the address of the master node it runs on.
//
// Order 1 keeps the addresses in values before the certificate hook (Order 5)
// builds its SAN list from them.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       proxyPodsSnapshot,
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{proxyNamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": proxyAppLabel,
				},
			},
			FilterFunc: proxyPodAddressFilter,
		},
	},
}, discoverProxyAddresses)

// proxyPodAddressFilter returns the address of a pod that serves traffic, and an
// empty string for every other pod. Pending, terminating and not yet ready pods
// have no address to publish.
func proxyPodAddressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}

	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return "", nil
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return pod.Status.PodIP, nil
		}
	}

	return "", nil
}

func discoverProxyAddresses(_ context.Context, input *go_hook.HookInput) error {
	addresses := set.NewFromSnapshot(input.Snapshots.Get(proxyPodsSnapshot))
	addresses.Delete("")

	// An empty list is a valid state: on the first converge no pod is running yet.
	// The certificate is then issued without addresses and reissued once they appear.
	input.Values.Set(proxyAddressesValuesPath, addresses.Slice())

	return nil
}
