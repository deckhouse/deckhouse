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

package tls_certificate

import (
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

const (
	certOutdateDuration = 4380 * time.Hour // 6 month
)

// DefaultSANs helper to generate list of sans for certificate
// you can also use helpers:
//    ClusterDomainSAN(value) to generate sans with respect of cluster domain (ex: "app.default.svc" with "cluster.local" value will give: app.default.svc.cluster.local
//    PublicDomainSAN(value)
func DefaultSANs(sans []string) SANsGenerator {
	return func(input *go_hook.HookInput) []string {
		clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()
		publicDomain := input.Values.Get("global.modules.publicDomainTemplate").String()

		for index, san := range sans {
			switch {
			case strings.HasPrefix(san, publicDomainPrefix) && publicDomain != "":
				sans[index] = getPublicDomainSAN(san, publicDomain)

			case strings.HasPrefix(san, clusterDomainPrefix) && clusterDomain != "":
				sans[index] = getClusterDomainSAN(san, clusterDomain)
			}
		}

		return sans
	}
}

type GenSelfSignedTLSHookConf struct {
	// SANs function which returns list of domain to include into cert. Use DefaultSANs helper
	SANs SANsGenerator

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
	//   FullValuesPathPrefix + .ca  - CA certificate
	//   FullValuesPathPrefix + .cert - TLS private key
	//   FullValuesPathPrefix + .key - TLS certificate
	// Example: FullValuesPathPrefix =  'prometheusMetricsAdapter.internal.adapter'
	// Values to store:
	// prometheusMetricsAdapter.internal.adapter.ca
	// prometheusMetricsAdapter.internal.adapter.cert
	// prometheusMetricsAdapter.internal.adapter.key
	// Data in values store as plain text
	// In helm templates you need use `b64enc` function to encode
	FullValuesPathPrefix string
}

func (gss GenSelfSignedTLSHookConf) generatePaths() (caPath, certPath, keyPath string) {
	prefix := strings.TrimSuffix(gss.FullValuesPathPrefix, ".")

	caPath = strings.Join([]string{prefix, "ca"}, ".")
	certPath = strings.Join([]string{prefix, "crt"}, ".")
	keyPath = strings.Join([]string{prefix, "key"}, ".")

	return
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
	caPath, certPath, keyPath := conf.generatePaths()

	return func(input *go_hook.HookInput) error {
		var cert certificate.Certificate
		var err error

		cn, sans := conf.CN, conf.SANs(input)

		if len(input.Snapshots["secret"]) == 0 {
			// No certificate in snapshot => generate a new one.
			// Secret will be updated by Helm.
			cert, err = generateNewTLS(input, cn, sans)
			if err != nil {
				return err
			}
		} else {
			// Certificate is in the snapshot => load it.
			cert = input.Snapshots["secret"][0].(certificate.Certificate)
			// update certificate if less than 6 month left. We create certificate for 10 years, so it looks acceptable
			// and we don't need to create Crontab schedule
			caOutdated, err := isOutdatedCA(cert.CA)
			if err != nil {
				return err
			}
			certOutdated, err := isOutdatedCert(cert.Cert, sans)
			if err != nil {
				return err
			}

			if caOutdated || certOutdated {
				cert, err = generateNewTLS(input, cn, sans)
				if err != nil {
					return err
				}
			}
		}

		// Note that []byte values will be encoded in base64. Use strings here!
		input.Values.Set(caPath, cert.CA)
		input.Values.Set(certPath, cert.Cert)
		input.Values.Set(keyPath, cert.Key)
		return nil
	}
}

func isOutdatedCert(certData string, desiredSANSs []string) (bool, error) {
	cert, err := certificate.ParseCertificate(certData)
	if err != nil {
		return false, err
	}

	if time.Until(cert.NotAfter) < certOutdateDuration {
		return true, nil
	}

	if !arrayAreEqual(desiredSANSs, cert.DNSNames) {
		return true, nil
	}

	return false, nil
}

func isOutdatedCA(ca string) (bool, error) {
	cert, err := certificate.ParseCertificate(ca)
	if err != nil {
		return false, err
	}

	if time.Until(cert.NotAfter) < certOutdateDuration {
		return true, nil
	}

	return false, nil
}

func generateNewTLS(input *go_hook.HookInput, cn string, sans []string) (certificate.Certificate, error) {
	ca, err := certificate.GenerateCA(input.LogEntry,
		cn,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	if err != nil {
		return certificate.Certificate{}, err
	}

	return certificate.GenerateSelfSignedCert(input.LogEntry,
		cn,
		ca,
		certificate.WithSANs(sans...),
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

// SANsGenerator function for generating sans
type SANsGenerator func(input *go_hook.HookInput) []string
