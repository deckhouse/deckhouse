/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"sort"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const disabledCacheBasePath = "nodeLocalDns.internal.disabledCache"

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "namespaces",
				Kind:       "Namespace",
				ApiVersion: "v1",
				FilterFunc: namespaceFilter,
				LabelSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{
						"node-local-dns.deckhouse.io/disable-cache": "",
					},
				},
			},
		},
	},
	func(_ context.Context, input *go_hook.HookInput) error {
		namespaceList, err := sdkobjectpatch.UnmarshalToStruct[corev1.Namespace](input.Snapshots, "namespaces")
		if err != nil {
			return fmt.Errorf("failed to decode namespaces snapshot: %w", err)
		}

		clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()
		if clusterDomain == "" {
			return fmt.Errorf("global.discovery.clusterDomain is empty")
		}

		disabledZones := make([]string, 0, len(namespaceList))
		for _, ns := range namespaceList {
			disabledZones = append(disabledZones, fmt.Sprintf("%s.%s", ns.Name, clusterDomain))
		}

		sort.Strings(disabledZones)
		if !input.Values.Get(disabledCacheBasePath).Exists() {
			input.Values.Set(disabledCacheBasePath, map[string]interface{}{})
		}
		input.Values.Set("nodeLocalDns.internal.disabledCache.zones", disabledZones)

		return nil
	},
)

func namespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ns := &corev1.Namespace{}

	err := sdk.FromUnstructured(obj, ns)
	if err != nil {
		return nil, err
	}

	return ns, nil
}
