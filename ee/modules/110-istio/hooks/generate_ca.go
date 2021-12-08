/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret_ca",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyIstioCAFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cacerts"},
			},
			NamespaceSelector: internal.NsSelector(),
		},
	},
}, generateCA)

func applyIstioCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned ca secret to secret: %v", err)
	}

	return internal.IstioCA{
		Cert:  string(secret.Data["ca-cert.pem"]),
		Key:   string(secret.Data["ca-key.pem"]),
		Chain: string(secret.Data["cert-chain.pem"]),
		Root:  string(secret.Data["root-cert.pem"]),
	}, nil
}

func generateCA(input *go_hook.HookInput) error {
	var istioCA internal.IstioCA

	if input.Values.Exists("istio.ca.cert") {
		istioCA.Cert = input.Values.Get("istio.ca.cert").String()
		istioCA.Key = input.Values.Get("istio.ca.key").String()
		if input.Values.Exists("istio.ca.chain") {
			istioCA.Chain = input.Values.Get("istio.ca.chain").String()
		} else {
			istioCA.Chain = istioCA.Cert
		}
		if input.Values.Exists("istio.ca.root") {
			istioCA.Root = input.Values.Get("istio.ca.root").String()
		} else {
			istioCA.Root = istioCA.Cert
		}
	} else {
		certs := input.Snapshots["secret_ca"]
		if len(certs) == 1 {
			var ok bool
			istioCA, ok = certs[0].(internal.IstioCA)
			if !ok {
				return fmt.Errorf("cannot convert certificate to certificate authority")
			}
		} else {
			selfSignedCA, err := certificate.GenerateCA(input.LogEntry, "d8-istio", certificate.WithGroups("d8-istio"), certificate.WithKeyRequest(&csr.KeyRequest{
				A: "rsa",
				S: 2048,
			}))
			istioCA.Cert = selfSignedCA.Cert
			istioCA.Key = selfSignedCA.Key
			istioCA.Chain = selfSignedCA.Cert
			istioCA.Root = selfSignedCA.Cert
			if err != nil {
				return err
			}
		}
	}

	input.Values.Set("istio.internal.ca.cert", istioCA.Cert)
	input.Values.Set("istio.internal.ca.key", istioCA.Key)
	input.Values.Set("istio.internal.ca.chain", istioCA.Chain)
	input.Values.Set("istio.internal.ca.root", istioCA.Root)

	return nil
}
