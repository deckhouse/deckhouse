/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"bashible-api-server-tls"},
			},

			FilterFunc: bashibleAPIServerTLSFilter,
		},
	},
}, genBashibleAPIServerCertsHandler)

func bashibleAPIServerTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["apiserver.crt"]),
		Key:  string(secret.Data["apiserver.key"]),
	}, nil
}

func genBashibleAPIServerCertsHandler(ctx context.Context, input *go_hook.HookInput) error {
	var cert certificate.Certificate
	var err error

	if len(input.Snapshots.Get("secret")) == 0 {
		// No certificate in snapshot => generate a new one.
		// Secret/bashible-api-server-tls will be updated by Helm.
		cert, err = generateNewBashibleCert(ctx, input)
		if err != nil {
			return err
		}
	} else {
		// Certificate is in the snapshot => load it.
		secrets := input.Snapshots.Get("secret")

		err = secrets[0].UnmarshalTo(&cert)
		if err != nil {
			return fmt.Errorf("failed to unmarshal first 'secret' snapshot")
		}
	}

	// Note that []byte values will be encoded in base64. Use strings here!
	input.Values.Set("nodeManager.internal.bashibleApiServerCA", cert.CA)
	input.Values.Set("nodeManager.internal.bashibleApiServerCrt", cert.Cert)
	input.Values.Set("nodeManager.internal.bashibleApiServerKey", cert.Key)
	return nil
}

func generateNewBashibleCert(_ context.Context, input *go_hook.HookInput) (certificate.Certificate, error) {
	ca, err := certificate.GenerateCA(input.Logger,
		"node-manager",
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	if err != nil {
		return certificate.Certificate{}, err
	}

	return certificate.GenerateSelfSignedCert(input.Logger,
		"node-manager",
		ca,
		certificate.WithSANs("127.0.0.1", "bashible-api.d8-cloud-instance-manager.svc"),
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry(87600*time.Hour),
		certificate.WithSigningDefaultUsage([]string{
			"signing",
			"key encipherment",
			"requestheader-client",
		}),
	)
}
