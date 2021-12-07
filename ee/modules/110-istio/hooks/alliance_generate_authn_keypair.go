/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

func applyKeypairFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert k8s secret to struct: %v", err)
	}

	return internal.Keypair{
		Pub:  string(secret.Data["pub.pem"]),
		Priv: string(secret.Data["priv.pem"]),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyKeypairFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-remote-authn-keypair"},
			},
			NamespaceSelector: internal.NsSelector(),
		},
	},
}, generateKeypair)

func generateKeypair(input *go_hook.HookInput) error {
	var keypair internal.Keypair

	secrets := input.Snapshots["secret"]
	if len(secrets) == 1 {
		var ok bool
		keypair, ok = secrets[0].(internal.Keypair)
		if !ok {
			return fmt.Errorf("cannot convert keypair in secret to struct")
		}
	} else {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}

		privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return err
		}
		privBlock := &pem.Block{
			Type:  "ED25519 PRIVATE KEY",
			Bytes: privBytes,
		}
		privPEM := pem.EncodeToMemory(privBlock)

		pubBytes, err := x509.MarshalPKIXPublicKey(pub)
		if err != nil {
			return err
		}
		pubBlock := &pem.Block{
			Type:  "ED25519 PUBLIC KEY",
			Bytes: pubBytes,
		}
		pubPEM := pem.EncodeToMemory(pubBlock)

		keypair = internal.Keypair{
			Pub:  string(pubPEM),
			Priv: string(privPEM),
		}
	}

	input.Values.Set("istio.internal.remoteAuthnKeypair.pub", keypair.Pub)
	input.Values.Set("istio.internal.remoteAuthnKeypair.priv", keypair.Priv)

	return nil
}
