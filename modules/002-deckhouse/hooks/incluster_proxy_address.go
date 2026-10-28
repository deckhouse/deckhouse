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
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/util/podutils"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	// Absent means "no answer": the template then renders the loopback fallback.
	inclusterProxyAddressPath = "deckhouse.internal.inclusterProxyAddress"

	inclusterProxySnapName = "incluster-proxy-pods"
	deckhouseSnapName      = "deckhouse-deployment"

	inclusterProxyHostname = "registry.d8-system.svc"
)

// Until the cluster is bootstrapped there is no cluster DNS, so this Pod resolves
// `registry.d8-system.svc` through a hostAlias. Pinned to the loopback address it only answers
// while the in-cluster registry proxy shares the node, which on a multi-master install it does
// not: the proxy is a single replica until the control plane is highly available.
//
// The proxy is on the host network, so a node address reaches it before CNI, and the name being
// resolved does not change, so its certificate still matches.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/deckhouse/incluster-proxy-address",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       inclusterProxySnapName,
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":       "registry-incluster-proxy",
					"component": "registry-service",
					"type":      "incluster-proxy",
				},
			},
			FilterFunc: filterInclusterProxyPod,
		},
		{
			Name:       deckhouseSnapName,
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			FilterFunc: filterDeckhouseInclusterProxyAlias,
		},
	},
}, discoverInclusterProxyAddress)

// filterInclusterProxyPod answers with the node address of a serving Pod, and an empty string
// otherwise — including for one on its way out, which can still report itself Ready.
func filterInclusterProxyPod(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	if err := sdk.FromUnstructured(obj, &pod); err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	if pod.DeletionTimestamp != nil || !podutils.IsPodReady(&pod) {
		return "", nil
	}

	return pod.Status.HostIP, nil
}

// filterDeckhouseInclusterProxyAlias answers with the address the alias currently resolves to,
// or an empty string when the Deployment renders none.
func filterDeckhouseInclusterProxyAlias(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var deployment appsv1.Deployment

	if err := sdk.FromUnstructured(obj, &deployment); err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	for _, alias := range deployment.Spec.Template.Spec.HostAliases {
		if slices.Contains(alias.Hostnames, inclusterProxyHostname) {
			return alias.IP, nil
		}
	}

	return "", nil
}

func discoverInclusterProxyAddress(_ context.Context, input *go_hook.HookInput) error {
	// Past bootstrap the Deployment renders no hostAlias at all.
	if input.Values.Get("global.clusterIsBootstrapped").Bool() {
		clearInclusterProxyAddress(input)
		return nil
	}

	snapshot := input.Snapshots.Get(inclusterProxySnapName)
	addresses := make([]string, 0, len(snapshot))

	for address, err := range sdkobjectpatch.SnapshotIter[string](snapshot) {
		if err != nil {
			return fmt.Errorf("cannot iterate over %s snapshot: %w", inclusterProxySnapName, err)
		}

		if address != "" {
			addresses = append(addresses, address)
		}
	}

	// Nothing is serving: the loopback fallback is as good as any address nothing answers on.
	if len(addresses) == 0 {
		clearInclusterProxyAddress(input)
		return nil
	}

	slices.Sort(addresses)

	// Reuse the address the alias already carries while it still reaches a proxy. Read off the
	// live Deployment, not off the values: publishing restarts this Pod, which loses the values,
	// and re-deriving from scratch would then restart it again.
	address := currentAlias(input)
	if !slices.Contains(addresses, address) {
		address = addresses[0]
	}

	// An unchanged value is not re-rendered, so it costs no restart.
	if input.Values.Get(inclusterProxyAddressPath).String() == address {
		return nil
	}

	input.Values.Set(inclusterProxyAddressPath, address)

	return nil
}

// currentAlias reads the address the rendered Deployment resolves the name to. Absent, unreadable
// or still on the loopback fallback all answer the same: nothing worth keeping.
func currentAlias(input *go_hook.HookInput) string {
	for alias, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get(deckhouseSnapName)) {
		if err == nil && alias != "" {
			return alias
		}
	}

	return ""
}

// clearInclusterProxyAddress removes the published address, and only if there is one.
func clearInclusterProxyAddress(input *go_hook.HookInput) {
	if input.Values.Exists(inclusterProxyAddressPath) {
		input.Values.Remove(inclusterProxyAddressPath)
	}
}
