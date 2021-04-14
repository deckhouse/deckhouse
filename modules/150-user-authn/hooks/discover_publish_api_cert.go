package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

type PublishAPICert struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

func (*PublishAPICert) ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := go_hook.ConvertUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return PublishAPICert{Name: obj.GetName(), Data: secret.Data["ca.crt"]}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-tls", "kubernetes-tls-customcertificate"},
			},
			Filterable: &PublishAPICert{},
		},
	},
}, discoverPublishAPICA)

func discoverPublishAPICA(input *go_hook.HookInput) error {
	secretPath := "userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA"

	caSecrets, ok := input.Snapshots["secret"]
	if !ok {
		return nil
	}

	if module.GetHTTPSMode("userAuthn", input) == "OnlyInURI" {
		if input.Values.Values.ExistsP(secretPath) {
			input.Values.Remove(secretPath)
		}
		return nil
	}

	secret, ok := caSecrets[0].(PublishAPICert)
	if !ok {
		return fmt.Errorf("cannot convert secret to publish api secret")
	}

	if len(secret.Data) > 0 {
		input.Values.Set(secretPath, string(secret.Data))
	} else if input.Values.Values.ExistsP(secretPath) {
		input.Values.Remove(secretPath)
	}

	return nil
}
