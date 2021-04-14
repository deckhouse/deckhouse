package hooks

import (
	"fmt"

	"github.com/chr4/pwgen"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type KubernetesSecret []byte

func (*KubernetesSecret) ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := go_hook.ConvertUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return secret.Data["secret"], nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kubernetes_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-dex-client-app-secret"},
			},
			Filterable: &KubernetesSecret{},
		},
	},
}, kubernetesDexClientAppSecret)

func kubernetesDexClientAppSecret(input *go_hook.HookInput) error {
	secretPath := "userAuthn.internal.kubernetesDexClientAppSecret"
	if input.Values.Values.ExistsP(secretPath) {
		return nil
	}

	kubernetesSecrets, ok := input.Snapshots["kubernetes_secret"]
	if ok && len(kubernetesSecrets) > 0 {
		secretContent, ok := kubernetesSecrets[0].([]byte)
		if !ok {
			return fmt.Errorf("cannot conver kubernetes secret to bytes")
		}

		input.Values.Set(secretPath, string(secretContent))
		return nil
	}

	input.Values.Set(secretPath, pwgen.AlphaNum(20))
	return nil
}
