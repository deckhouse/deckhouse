package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyCloudProviderSecretFilter loads data section from Secret and tries to decode json in all top fields.
func applyCloudProviderSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return DecodeDataFromSecret(obj)
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-node-manager-cloud-provider",
			}},
			FilterFunc: applyCloudProviderSecretFilter,
		},
	},
}, discoverCloudProviderHandler)

func discoverCloudProviderHandler(input *go_hook.HookInput) error {
	secret := input.Snapshots["cloud_provider_secret"]
	if len(secret) == 0 {
		if input.Values.Exists("nodeManager.internal.cloudProvider") {
			input.Values.Remove("nodeManager.internal.cloudProvider")
		}
		return nil
	}
	input.Values.Set("nodeManager.internal.cloudProvider", secret[0])
	return nil
}
