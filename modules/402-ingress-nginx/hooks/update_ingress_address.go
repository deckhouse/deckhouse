package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type loadBalancerService struct {
	name     string
	hostname string
	ip       string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/ingress-nginx",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress-loadbalancer-service",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"deckhouse-service-type": "provider-managed",
				},
			},
			FilterFunc: filterIngressServiceAddress,
		},
	},
}, updateIngressAddress)

func filterIngressServiceAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc v12.Service

	if err := sdk.FromUnstructured(obj, &svc); err != nil {
		return nil, err
	}
	if svc.Status.LoadBalancer.Ingress != nil && len(svc.Status.LoadBalancer.Ingress) != 0 {
		return loadBalancerService{
			name:     svc.Labels["name"],
			ip:       svc.Status.LoadBalancer.Ingress[0].IP,
			hostname: svc.Status.LoadBalancer.Ingress[0].Hostname,
		}, nil
	}
	return nil, nil
}

func updateIngressAddress(input *go_hook.HookInput) error {
	snaps := input.Snapshots["ingress-loadbalancer-service"]
	for _, snap := range snaps {
		if snap == nil {
			continue
		}
		svc := snap.(loadBalancerService)
		patch := map[string]interface{}{
			"status": map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ip":       svc.ip,
					"hostname": svc.hostname,
				},
			},
		}
		input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "IngressNginxController",
			"", svc.name, object_patch.IgnoreMissingObject())
	}
	return nil
}
