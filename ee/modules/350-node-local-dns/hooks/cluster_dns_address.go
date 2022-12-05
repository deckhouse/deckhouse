/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "dns_redirect_cluster_ip",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: v1.LabelSelectorOpIn,
						Values:   []string{"coredns-redirect"},
					},
				},
			},
			FilterFunc: applyDNSServiceIPFilter,
		},
	},
}, discoveryDNSAddress)

func applyDNSServiceIPFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1core.Service
	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return "", err
	}

	return service.Spec.ClusterIP, nil
}

func discoveryDNSAddress(input *go_hook.HookInput) error {
	dnsRedirAddressSnap := input.Snapshots["dns_redirect_cluster_ip"]
	dnsRedirAddress := extractDNSAddressFromSnapshot(dnsRedirAddressSnap)
	if dnsRedirAddress == "" {
		return fmt.Errorf("DNS redirect address not found")
	}

	input.Values.Set("nodeLocalDns.internal.clusterDNSRedirectAddress", dnsRedirAddress)

	return nil
}

func extractDNSAddressFromSnapshot(snap []go_hook.FilterResult) string {
	for _, addrRaw := range snap {
		addr := addrRaw.(string)
		if addr == "None" || addr == "" {
			continue
		}

		return addr
	}

	return ""
}
