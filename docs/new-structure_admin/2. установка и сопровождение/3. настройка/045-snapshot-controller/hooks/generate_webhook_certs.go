/*
Copyright 2022 Flant JSC

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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

const (
	namespace   = "d8-snapshot-controller"
	serviceName = "snapshot-validation-webhook"
	serviceHost = serviceName + "." + namespace + ".svc"
	secretName  = "snapshot-validation-webhook-certs"
	certPath    = "snapshotController.internal.webhookCert"
)

type CertSnapshot struct {
	Name string
	Cert certificate.Certificate
}

func applyCertsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned certificate secret to secret: %v", err)
	}

	cs := &CertSnapshot{
		Name: secret.Name,
		Cert: certificate.Certificate{
			CA:   string(secret.Data["ca.crt"]),
			Key:  string(secret.Data["tls.key"]),
			Cert: string(secret.Data["tls.crt"]),
		}}

	return cs, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "certs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{secretName},
			},
			FilterFunc: applyCertsFilter,
		},
	},
}, generateSelfSignedCertificates)

func generateSelfSignedCertificates(input *go_hook.HookInput) error {
	var caCert certificate.Authority
	var cert certificate.Certificate

	snaps := input.Snapshots["certs"]
	for _, snap := range snaps {
		s := snap.(*CertSnapshot)
		cert = s.Cert
	}

	if cert.CA == "" || cert.Cert == "" || cert.Key == "" {
		var err error
		caCert, err = certificate.GenerateCA(input.LogEntry, "snapshot-validation-webhook-ca")
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}

		cert, err = certificate.GenerateSelfSignedCert(input.LogEntry,
			"snapshot-validation-webhook",
			caCert,
			certificate.WithSigningDefaultExpiry(87600*time.Hour),
			certificate.WithSANs(
				serviceName,
				serviceHost,
				"localhost",
				"::1",
				"127.0.0.1",
			),
		)
		if err != nil {
			return fmt.Errorf("cannot generate certificate: %v", err)
		}
	}

	input.Values.Set(certPath, cert)
	return nil
}
