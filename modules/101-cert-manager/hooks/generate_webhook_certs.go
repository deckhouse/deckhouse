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
	var name = secret.Name

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
	const cn = "cert-manager.webhook.ca"
	ca, err := certificate.GenerateCA(logEntry, cn, func(r *csr.CertificateRequest) {
		r.KeyRequest = &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}
		r.Hosts = []string{"cert-manager.webhook.ca"}
		r.Names = []csr.Name{
			{O: "cert-manager.system"},
		}
	})

	if err != nil {
		return nil, fmt.Errorf("cannot generate CA: %v", err)
	}

	return &ca, nil
}

func genWebhookTLS(input *go_hook.HookInput, ca *certificate.Authority) (*certificate.Certificate, error) {
	const cn = "cert-manager-webhook"
	hosts := []string{
		"cert-manager-webhook",
		"cert-manager-webhook.d8-cert-manager",
		"cert-manager-webhook.d8-cert-manager.svc",
	}

	tls, err := certificate.GenerateSelfSignedCert(input.LogEntry, cn, hosts, *ca, func(r *csr.CertificateRequest) {
		r.Names = []csr.Name{
			{O: "cert-manager.system"},
		}
		r.KeyRequest = &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}
	})

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
	}

	input.Values.Set("certManager.internal.webhookCACrt", caAuthority.Cert)
	input.Values.Set("certManager.internal.webhookCAKey", caAuthority.Key)

	input.Values.Set("certManager.internal.webhookCrt", tlsAuthority.Cert)
	input.Values.Set("certManager.internal.webhookKey", tlsAuthority.Key)

	return nil
}
