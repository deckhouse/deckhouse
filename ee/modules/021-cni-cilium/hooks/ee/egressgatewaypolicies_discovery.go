/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	eeCrd "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium/egress-policy-discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "egressgatewaypolicies",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "EgressGatewayPolicy",
			FilterFunc: applyEgressGatewayPolicyFilter,
		},
	},
}, handleEgressGatewayPolicies)

func applyEgressGatewayPolicyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var egp eeCrd.EgressGatewayPolicy

	err := sdk.FromUnstructured(obj, &egp)
	if err != nil {
		return nil, err
	}

	return EgressGatewayPolicyInfo{
		Name:              egp.Name,
		EgressGatewayName: egp.Spec.EgressGatewayName,
		Selectors:         egp.Spec.Selectors,
		DestinationCIDRs:  egp.Spec.DestinationCIDRs,
		ExcludedCIDRs:     egp.Spec.ExcludedCIDRs,
	}, err
}

func handleEgressGatewayPolicies(input *go_hook.HookInput) error {
	input.Values.Set("cniCilium.internal.egressGatewayPolicies", input.Snapshots["egressgatewaypolicies"])

	return nil
}

type EgressGatewayPolicyInfo struct {
	Name              string           `json:"name"`
	EgressGatewayName string           `json:"egressGatewayName"`
	Selectors         []eeCrd.Selector `json:"selectors"`
	DestinationCIDRs  []string         `json:"destinationCIDRs"`
	ExcludedCIDRs     []string         `json:"excludedCIDRs"`
}
