/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"crypto/sha256"
	"fmt"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	configMapSnapshotName = "configMapNodeLocalDNS"
	podsSnapshotName      = "podsNodeLocalDNS"

	configHashAnnotation = "node-local-dns.deckhouse.io/config-hash"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnAfterHelm: &go_hook.OrderedConfig{},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       configMapSnapshotName,
				Kind:       "ConfigMap",
				ApiVersion: "v1",
				FilterFunc: configMapFilter,
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				NameSelector: &types.NameSelector{
					MatchNames: []string{"node-local-dns"},
				},
			},
			{
				Name:       podsSnapshotName,
				Kind:       "Pod",
				ApiVersion: "v1",
				FilterFunc: podFilter,
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				LabelSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{
						"app":     "node-local-dns",
						"k8s-app": "node-local-dns",
					},
				},
			},
		},
	},
	func(_ context.Context, input *go_hook.HookInput) error {
		configMaps, err := sdkobjectpatch.UnmarshalToStruct[corev1.ConfigMap](input.Snapshots, configMapSnapshotName)
		if err != nil {
			return fmt.Errorf("failed to decode ConfigMap snapshot: %w", err)
		}

		if len(configMaps) == 0 {
			return nil
		}

		corefile := configMaps[0].Data["Corefile"]
		newHash := fmt.Sprintf("%x", sha256.Sum256([]byte(corefile)))

		prevHash := input.Values.Get("nodeLocalDns.internal.disabledCache.hash").String()
		if newHash == prevHash {
			return nil
		}

		input.Values.Set("nodeLocalDns.internal.disabledCache.hash", newHash)

		pods, err := sdkobjectpatch.UnmarshalToStruct[corev1.Pod](input.Snapshots, podsSnapshotName)
		if err != nil {
			return fmt.Errorf("failed to decode Pods snapshot: %w", err)
		}

		for _, pod := range pods {
			if pod.Annotations != nil && pod.Annotations[configHashAnnotation] == newHash {
				continue
			}

			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						configHashAnnotation: newHash,
					},
				},
			}

			input.PatchCollector.PatchWithMerge(patch, "v1", "Pod", pod.Namespace, pod.Name)
		}

		return nil
	},
)

func configMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &corev1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, err
	}

	return cm, nil
}

func podFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}
	if err := sdk.FromUnstructured(obj, pod); err != nil {
		return nil, err
	}

	return pod, nil
}
