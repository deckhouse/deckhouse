package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

type Cert struct{}

func applyCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned ca secret to secret: %v", err)
	}

	return certificate.Authority{
		Key:  string(secret.Data["tls.key"]),
		Cert: string(secret.Data["tls.crt"]),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cert",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-api-ca-key-pair"},
			},
			FilterFunc: applyCertFilter,
		},
	},
}, generateSelfSignedCA)

func generateSelfSignedCA(input *go_hook.HookInput) error {
	const (
		certPath = "userAuthn.internal.selfSignedCA.cert"
		keyPath  = "userAuthn.internal.selfSignedCA.key"
	)

	publishAPIEnabled := input.Values.Values.Path("userAuthn.publishAPI.enable").Data().(bool)
	publishAPIMode := input.Values.Values.Path("userAuthn.publishAPI.https.mode").Data().(string)

	if !publishAPIEnabled && publishAPIMode != "SelfSigned" {
		if input.Values.Values.ExistsP(certPath) {
			input.Values.Remove(certPath)
		}

		if input.Values.Values.Exists(keyPath) {
			input.Values.Remove(keyPath)
		}
		return nil
	}

	var sefSignedCA certificate.Authority

	certs := input.Snapshots["cert"]
	if len(certs) == 1 {
		var ok bool
		sefSignedCA, ok = certs[0].(certificate.Authority)
		if !ok {
			return fmt.Errorf("cannot convert sefsigned certificate to certificate authority")
		}
	} else {
		var err error
		sefSignedCA, err = certificate.GenerateCA(input.LogEntry, "kubernetes-api-selfsigned-ca")
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}
	}

	input.Values.Set(certPath, sefSignedCA.Cert)
	input.Values.Set(keyPath, sefSignedCA.Key)
	return nil
}
