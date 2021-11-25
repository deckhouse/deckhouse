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
	"fmt"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

const (
	webhookSnapshotTLS = "secrets"

	webhookSecretCa  = "cert-manager-webhook-ca"
	webhookSecretTLS = "cert-manager-webhook-tls"
)

type webHookAuthority struct {
	certificate.Authority
	Name string
}

func applyWebhookSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert to secret webhook CA from secret input: %v", err)
	}

	var certField string
	var keyField string
	name := secret.Name

	switch name {
	case webhookSecretCa:
		certField = "ca.crt"
		keyField = "tls.key"
	case webhookSecretTLS:
		certField = "tls.crt"
		keyField = "tls.key"
	default:
		return nil, nil
	}

	data := secret.Data
	return webHookAuthority{
		Authority: certificate.Authority{
			Cert: string(data[certField]),
			Key:  string(data[keyField]),
		},
		Name: name,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		// getting all from namespace, because we dont want more informers and
		// TODO maybe we do not need save Ca keys in secrets
		{
			Name:              webhookSnapshotTLS,
			ApiVersion:        "v1",
			Kind:              "Secret",
			FilterFunc:        applyWebhookSecretFilter,
			NamespaceSelector: internal.NsSelector(),
		},
	},
}, genWebhookCerts)

func genWebhookCa(logEntry *logrus.Entry) (*certificate.Authority, error) {
	const cn = "cert-manager-webhook"
	ca, err := certificate.GenerateCA(logEntry, cn, func(r *csr.CertificateRequest) {
		r.KeyRequest = &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}
		r.Hosts = []string{
			"cert-manager-webhook.d8-cert-manager.svc",
			"legacy-cert-manager-webhook.d8-cert-manager.svc",
			"annotations-converter-webhook.d8-cert-manager.svc",
			"cert-manager-webhook.d8-cert-manager",
			"legacy-cert-manager-webhook.d8-cert-manager",
			"annotations-converter-webhook.d8-cert-manager",
			"cert-manager-webhook",
			"legacy-cert-manager-webhook",
			"annotations-converter-webhook",
		}
		r.Names = []csr.Name{
			{O: "cert-manager-webhook.d8-cert-manager"},
			{O: "legacy-cert-manager-webhook.d8-cert-manager"},
			{O: "annotations-converter-webhook.d8-cert-manager"},
		}
	})
	if err != nil {
		return nil, fmt.Errorf("cannot generate CA: %v", err)
	}

	return &ca, nil
}

func genWebhookTLS(input *go_hook.HookInput, ca *certificate.Authority) (*certificate.Certificate, error) {
	tls, err := certificate.GenerateSelfSignedCert(input.LogEntry,
		"cert-manager-webhook",
		*ca,
		certificate.WithGroups(
			"cert-manager.d8-cert-manager",
			"legacy-cert-manager.d8-cert-manager",
			"annotations-converter-webhook.d8-cert-manager",
		),
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithSANs(
			"cert-manager-webhook.d8-cert-manager.svc",
			"legacy-cert-manager-webhook.d8-cert-manager.svc",
			"annotations-converter-webhook.d8-cert-manager.svc",
			"cert-manager-webhook.d8-cert-manager",
			"legacy-cert-manager-webhook.d8-cert-manager",
			"annotations-converter-webhook.d8-cert-manager",
			"cert-manager-webhook",
			"legacy-cert-manager-webhook",
			"annotations-converter-webhook",
		),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot generate TLS: %v", err)
	}

	return &tls, err
}

func genWebhookCerts(input *go_hook.HookInput) error {
	var caAuthority *certificate.Authority
	var tlsAuthority *certificate.Authority

	for _, v := range input.Snapshots[webhookSnapshotTLS] {
		if v == nil {
			continue
		}

		a := v.(webHookAuthority)
		switch a.Name {
		case webhookSecretCa:
			caAuthority = &a.Authority
		case webhookSecretTLS:
			tlsAuthority = &a.Authority
		}
	}

	if caAuthority == nil || tlsAuthority == nil {
		var err error
		caAuthority, err = genWebhookCa(input.LogEntry)
		if err != nil {
			return err
		}

		tls, err := genWebhookTLS(input, caAuthority)
		if err != nil {
			return err
		}

		tlsAuthority = &certificate.Authority{
			Cert: tls.Cert,
			Key:  tls.Key,
		}
	} else {
		ca, _, err := certificate.ParseCertificatesFromPEM(caAuthority.Cert, tlsAuthority.Cert, tlsAuthority.Key)
		if err != nil {
			return err
		}
		// migrate from previous legacy version
		// this 'else' branch could be removed when we will remove legacy cert-manager
		if ca.Subject.CommonName != "cert-manager-webhook" || !has(ca.Subject.Organization, "annotations-converter-webhook.d8-cert-manager") {
			var err error
			caAuthority, err = genWebhookCa(input.LogEntry)
			if err != nil {
				return err
			}

			tls, err := genWebhookTLS(input, caAuthority)
			if err != nil {
				return err
			}

			tlsAuthority = &certificate.Authority{
				Cert: tls.Cert,
				Key:  tls.Key,
			}
		}
	}

	input.Values.Set("certManager.internal.webhookCACrt", caAuthority.Cert)
	input.Values.Set("certManager.internal.webhookCAKey", caAuthority.Key)

	input.Values.Set("certManager.internal.webhookCrt", tlsAuthority.Cert)
	input.Values.Set("certManager.internal.webhookKey", tlsAuthority.Key)

	return nil
}

func has(s []string, key string) bool {
	for _, v := range s {
		if v == key {
			return true
		}
	}

	return false
}
