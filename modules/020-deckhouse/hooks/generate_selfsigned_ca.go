/*
Copyright 2021 Flant CJSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

func applyCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned certificate secret to secret: %v", err)
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
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
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"webhook-handler-certs"},
			},
			FilterFunc: applyCertFilter,
		},
	},
}, generateSelfSignedCertificate)

func generateSelfSignedCertificate(input *go_hook.HookInput) error {
	const (
		webhookServiceHost = "webhook-handler.d8-system.svc"

		webhookHandlerCertPath = "deckhouse.internal.webhookHandlerCert.crt"
		webhookHandlerKeyPath  = "deckhouse.internal.webhookHandlerCert.key"
		webhookHandlerCAPath   = "deckhouse.internal.webhookHandlerCert.ca"
	)

	var sefSignedCert certificate.Certificate

	certs := input.Snapshots["cert"]
	if len(certs) == 1 {
		var ok bool
		sefSignedCert, ok = certs[0].(certificate.Certificate)
		if !ok {
			return fmt.Errorf("cannot convert sefsigned certificate to certificate")
		}
	} else {
		var err error
		sefSignedCA, err := certificate.GenerateCA(input.LogEntry, webhookServiceHost)
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}

		webhookServiceFQDN := fmt.Sprintf(
			"%s.%s",
			webhookServiceHost,
			input.Values.Get("global.discovery.clusterDomain").String(),
		)
		sefSignedCert, err = certificate.GenerateSelfSignedCert(input.LogEntry,
			"webhook-handler",
			sefSignedCA,
			certificate.WithSANs(
				webhookServiceHost,
				webhookServiceFQDN,
				"validating-"+webhookServiceHost,
				"conversion-"+webhookServiceHost,
				"validating-"+webhookServiceFQDN,
				"conversion-"+webhookServiceFQDN,
			),
		)
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned certificate: %v", err)
		}
	}

	input.Values.Set(webhookHandlerCertPath, sefSignedCert.Cert)
	input.Values.Set(webhookHandlerKeyPath, sefSignedCert.Key)
	input.Values.Set(webhookHandlerCAPath, sefSignedCert.CA)
	return nil
}
