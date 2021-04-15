package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DexProvider map[string]interface{}

func applyDexProviderFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from dex provider: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("dex provider has no spec field")
	}

	spec["id"] = obj.GetName()
	return DexProvider(spec), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "providers",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "DexProvider",
			FilterFunc: applyDexProviderFilter,
		},
	},
}, getDexProviders)

func getDexProviders(input *go_hook.HookInput) error {
	providers, ok := input.Snapshots["providers"]
	if !ok {
		input.Values.Set("userAuthn.internal.providers", []interface{}{})
		return nil
	}

	input.Values.Set("userAuthn.internal.providers", providers)
	return nil
}
