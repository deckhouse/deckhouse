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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

type GenSelfSignedTLSHookConf struct {
	// SANs - list of domains to include into certificate
	SANs []string
	// SANsGenerator dynamic SANs generator based in hook input (e.x.: values like cluster domain)
	SANsGenerator func(input *go_hook.HookInput) []string

	// CN - Certificate common Name
	// often it is module name
	CN string

	// Namespace - namespace for TLS secret
	Namespace string
	// TLSSecretName - TLS secret name
	// secret must be TLS secret type https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets
	// CA certificate MUST set to ca.crt key
	TLSSecretName string

	// FullValuesPathPrefix - prefix full path to store CA certificate TLS private key and cert
	// full paths will be
	//   FullValuesPathPrefix + CA  - CA certificate
	//   FullValuesPathPrefix + Pem - TLS private key
	//   FullValuesPathPrefix + Key - TLS certificate
	// Example: FullValuesPathPrefix =  'prometheusMetricsAdapter.internal.adapter'
	// Values to store:
	// prometheusMetricsAdapter.internal.adapterCA
	// prometheusMetricsAdapter.internal.adapterPem
	// prometheusMetricsAdapter.internal.adapterKey
	// Data in values store as plain text
	// In helm templates you need use `b64enc` function to encode
	FullValuesPathPrefix string

	// You can set Paths for internal values explicitly or use FullValuesPathPrefix
	CAValuesPath   string
	CertValuesPath string
	KeyValuesPath  string
}

// if values path is set explicitly - use them; if not - generate them from prefix
func (gss *GenSelfSignedTLSHookConf) fillValuesPath() {
	if gss.CAValuesPath != "" && gss.KeyValuesPath != "" && gss.CertValuesPath != "" {
		return
	}

	// backward compatibility
	gss.CAValuesPath = gss.FullValuesPathPrefix + "CA"
	gss.CertValuesPath = gss.FullValuesPathPrefix + "Pem"
	gss.KeyValuesPath = gss.FullValuesPathPrefix + "Key"
}

// RegisterInternalTLSHook
// Register hook which save tls cert in values from secret.
// If secret is not created hook generate CA with long expired time
// and generate tls cert for passed domains signed with generated CA.
// That CA cert and TLS cert and private key MUST save in secret with helm.
// Otherwise, every d8 restart will generate new tls cert.
// Tls cert also has long expired time same as CA 87600h == 10 years.
// Therese tls cert often use for in cluster https communication
// with service which order tls
// Clients need to use CA cert for verify connection
func RegisterInternalTLSHook(conf GenSelfSignedTLSHookConf) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "secret",
				ApiVersion: "v1",
				Kind:       "Secret",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{conf.Namespace},
					},
				},
				NameSelector: &types.NameSelector{
					MatchNames: []string{conf.TLSSecretName},
				},

				FilterFunc: tlsFilter,
			},
		},
	}, genSelfSignedTLS(conf))
}

func tlsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func genSelfSignedTLS(conf GenSelfSignedTLSHookConf) func(input *go_hook.HookInput) error {
	conf.fillValuesPath()
	return func(input *go_hook.HookInput) error {
		var cert certificate.Certificate
		var err error

		if len(input.Snapshots["secret"]) == 0 {
			// No certificate in snapshot => generate a new one.
			// Secret will be updated by Helm.
			cert, err = generateNewTLS(input, conf)
			if err != nil {
				return err
			}
		} else {
			// Certificate is in the snapshot => load it.
			cert = input.Snapshots["secret"][0].(certificate.Certificate)
			// update certificate if less than 6 month left. We create certificate for 10 years, so it looks acceptable
			// and we don't need to create Crontab schedule
			expiring, err := certificate.IsCertificateExpiringSoon([]byte(cert.Cert), 4380*time.Hour) // 6 month
			if err != nil {
				return err
			}
			if expiring {
				cert, err = generateNewTLS(input, conf)
				if err != nil {
					return err
				}
			}
		}

		// Note that []byte values will be encoded in base64. Use strings here!
		input.Values.Set(conf.CAValuesPath, cert.CA)
		input.Values.Set(conf.CertValuesPath, cert.Cert)
		input.Values.Set(conf.KeyValuesPath, cert.Key)
		return nil
	}
}

func generateNewTLS(input *go_hook.HookInput, conf GenSelfSignedTLSHookConf) (certificate.Certificate, error) {
	if conf.SANsGenerator != nil {
		conf.SANs = append(conf.SANs, conf.SANsGenerator(input)...)
	}

	ca, err := certificate.GenerateCA(input.LogEntry,
		conf.CN,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	if err != nil {
		return certificate.Certificate{}, err
	}

	return certificate.GenerateSelfSignedCert(input.LogEntry,
		conf.CN,
		ca,
		certificate.WithSANs(conf.SANs...),
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
