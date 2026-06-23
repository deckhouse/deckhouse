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
	"context"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	certificatesv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/net"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

const (
	caExpiryDurationStr  = "87600h"                    // 10 years
	certExpiryDuration   = (24 * time.Hour) * 365 * 10 // 10 years
	certOutdatedDuration = (24 * time.Hour) * 365 / 2  // 6 month, just enough to renew certificate

	// certificate encryption algorithm
	keyAlgorithm = "ecdsa"
	keySize      = 256

	// caOrganization is placed in the CA's Subject DN (O=). Together with an
	// OU (see GenSelfSignedTLSHookConf.CAOrganizationalUnit) it guarantees that
	// the CA's Subject DN is strictly different from the leaf's Subject DN.
	// Per RFC 5280 §4.1.2.6 a leaf must have Issuer == Subject(CA) and
	// Subject(leaf) != Subject(CA); when those collide OpenSSL classifies the
	// leaf as a depth-0 self-signed certificate
	// (X509_V_ERR_DEPTH_ZERO_SELF_SIGNED_CERT) and refuses to chain it to the
	// CA, even though the leaf is in fact signed by a separate key. Go's
	// crypto/x509 (and thus kube-apiserver) is more lenient, so legacy
	// certificates issued without this differentiation continue to validate
	// for Go-based clients, but external scanners (Trivy, MaxPatrol) and
	// strict TLS stacks (Java keystore, openssl verify) reject them.
	caOrganization = "Deckhouse"

	// namespacePrefix is the conventional Deckhouse namespace prefix that is
	// stripped to derive a default CA OU when the caller has not supplied one.
	namespacePrefix = "d8-"

	SnapshotKey = "secret"
)

// defaultUsages is the minimum set of cfssl usages for a TLS server
// certificate. The legacy default contained the pseudo-usage
// "requestheader-client" which is not present in cfssl's signing/extkeyusage
// maps; cfssl-signer silently drops it and emits a certificate without any
// ExtendedKeyUsage. Strict validators (Trivy, MaxPatrol with EKU checks, Java
// keystores) reject such certificates.
var defaultUsages = []string{
	"signing",
	"key encipherment",
	"server auth",
}

// DefaultSANs helper to generate list of sans for certificate
// you can also use helpers:
//
//	ClusterDomainSAN(value) to generate sans with respect of cluster domain (e.g.: "app.default.svc" with "cluster.local" value will give: app.default.svc.cluster.local
//	PublicDomainSAN(value)
func DefaultSANs(sans []string) SANsGenerator {
	return func(_ context.Context, input *go_hook.HookInput) []string {
		res := make([]string, 0, len(sans))

		clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()
		publicDomain := input.Values.Get("global.modules.publicDomainTemplate").String()

		for _, san := range sans {
			switch {
			case strings.HasPrefix(san, publicDomainPrefix) && publicDomain != "":
				san = getPublicDomainSAN(san, publicDomain)

			case strings.HasPrefix(san, clusterDomainPrefix) && clusterDomain != "":
				san = getClusterDomainSAN(san, clusterDomain)
			}

			res = append(res, san)
		}
		return res
	}
}

type GenSelfSignedTLSHookConf struct {
	// SANs function which returns list of domain to include into cert. Use DefaultSANs helper
	SANs SANsGenerator

	// CN - Certificate common Name
	// often it is module name
	CN string

	// CAOrganizationalUnit is set as the OU on the CA's Subject DN to ensure
	// CA and leaf certificates have distinct Subject DNs (see caOrganization
	// for the rationale). Set this to the Deckhouse module name (e.g.
	// "node-manager", "loki"). When left empty the value is derived from
	// Namespace by stripping the "d8-" prefix; if Namespace does not have the
	// prefix or is empty, CN is used as a last-resort fallback. The leaf
	// certificate is intentionally signed CN-only — do NOT pass the same OU
	// to GenerateSelfSignedCert, otherwise Subject(leaf) == Subject(CA) and
	// the depth-0 self-signed collision returns.
	CAOrganizationalUnit string

	// Namespace - namespace for TLS secret
	Namespace string
	// TLSSecretName - TLS secret name
	// secret must be TLS secret type https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets
	// CA certificate MUST set to ca.crt key
	TLSSecretName string

	// Usages specifies valid usage contexts for keys.
	// See: https://tools.ietf.org/html/rfc5280#section-4.2.1.3
	//      https://tools.ietf.org/html/rfc5280#section-4.2.1.12
	//
	// Always pass an explicit set that contains "server auth" for server
	// certificates and "client auth" for mTLS clients. Never pass
	// "requestheader-client" — it is not a valid cfssl usage and silently
	// produces certificates with empty ExtendedKeyUsage.
	//
	// Reference sets:
	//   server cert:                ["signing", "key encipherment", "server auth"]
	//   mTLS client cert:           ["signing", "key encipherment", "client auth"]
	//   mTLS server validating
	//   a client (dual-purpose):    ["signing", "key encipherment", "server auth", "client auth"]
	Usages []certificatesv1.KeyUsage

	// FullValuesPathPrefix - prefix full path to store CA certificate TLS private key and cert
	// full paths will be
	//   FullValuesPathPrefix + .ca  - CA certificate
	//   FullValuesPathPrefix + .crt - TLS private key
	//   FullValuesPathPrefix + .key - TLS certificate
	// Example: FullValuesPathPrefix =  'prometheusMetricsAdapter.internal.adapter'
	// Values to store:
	// prometheusMetricsAdapter.internal.adapter.ca
	// prometheusMetricsAdapter.internal.adapter.crt
	// prometheusMetricsAdapter.internal.adapter.key
	// Data in values store as plain text
	// In helm templates you need use `b64enc` function to encode
	FullValuesPathPrefix string

	// BeforeHookCheck runs check function before hook execution. Function should return boolean 'continue' value
	// if return value is false - hook will stop its execution
	// if return value is true - hook will continue
	BeforeHookCheck func(_ context.Context, input *go_hook.HookInput) bool
}

func (gss GenSelfSignedTLSHookConf) path() string {
	return strings.TrimSuffix(gss.FullValuesPathPrefix, ".")
}

// caOU returns the Organizational Unit placed on the CA's Subject DN.
// Priority: explicit CAOrganizationalUnit → Namespace with the "d8-" prefix
// stripped → CN. The result is always non-empty when CN is non-empty, which
// is what differentiates the CA's Subject from the leaf's CN-only Subject.
func (gss GenSelfSignedTLSHookConf) caOU() string {
	if gss.CAOrganizationalUnit != "" {
		return gss.CAOrganizationalUnit
	}
	if strings.HasPrefix(gss.Namespace, namespacePrefix) {
		if trimmed := strings.TrimPrefix(gss.Namespace, namespacePrefix); trimmed != "" {
			return trimmed
		}
	}
	return gss.CN
}

type certValues struct {
	CA  string `json:"ca"`
	Crt string `json:"crt"`
	Key string `json:"key"`
}

// The certificate mapping "cert" -> "crt". We are migrating to "crt" naming for certificates
// in values.
func convCertToValues(cert certificate.Certificate) certValues {
	return certValues{
		CA:  cert.CA,
		Crt: cert.Cert,
		Key: cert.Key,
	}
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
				Name:       SnapshotKey,
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
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func genSelfSignedTLS(conf GenSelfSignedTLSHookConf) func(ctx context.Context, input *go_hook.HookInput) error {
	var usages []string
	if conf.Usages == nil {
		usages = append(usages, defaultUsages...)
	} else {
		for _, v := range conf.Usages {
			usages = append(usages, string(v))
		}
	}

	return func(ctx context.Context, input *go_hook.HookInput) error {
		if conf.BeforeHookCheck != nil {
			passed := conf.BeforeHookCheck(ctx, input)
			if !passed {
				return nil
			}
		}

		var cert certificate.Certificate
		var err error

		cn, sans := conf.CN, conf.SANs(ctx, input)

		certs, err := sdkobjectpatch.UnmarshalToStruct[certificate.Certificate](input.Snapshots, SnapshotKey)
		if err != nil {
			return fmt.Errorf("failed to unmarshal secret snapshot: %w", err)
		}

		if len(certs) == 0 {
			// No certificate in snapshot => generate a new one.
			// Secret will be updated by Helm.
			cert, err = generateNewSelfSignedTLS(input, conf, sans, usages)
			if err != nil {
				return err
			}
		} else {
			// Certificate is in the snapshot => load it.
			cert = certs[0]
			// update certificate if less than 6 month left. We create certificate for 10 years, so it looks acceptable
			// and we don't need to create Crontab schedule
			caOutdated, err := isOutdatedCA(cert.CA)
			if err != nil {
				input.Logger.Error(err.Error())
			}

			certOutdated, err := isIrrelevantCert(cert.Cert, cn, sans)
			if err != nil {
				input.Logger.Error(err.Error())
			}

			// In case of errors, both these flags are false to avoid regeneration loop for the
			// certificate.
			if caOutdated || certOutdated {
				cert, err = generateNewSelfSignedTLS(input, conf, sans, usages)
				if err != nil {
					return err
				}
			}
		}

		input.Values.Set(conf.path(), convCertToValues(cert))
		return nil
	}
}

// isIrrelevantCert decides whether an existing leaf certificate must be
// re-issued. It triggers a re-issue when:
//
//   - the certificate is approaching its expiry,
//   - the configured CN no longer matches Subject.CommonName,
//   - the leaf was issued without Subject DN differentiation
//     (Subject == Issuer), which produces certificates rejected by openssl,
//   - the leaf has no ExtendedKeyUsage extension, which happens when a hook
//     was previously configured with the legacy "requestheader-client"
//     pseudo-usage (cfssl silently drops it),
//   - the SAN set drifted from the desired one.
func isIrrelevantCert(certData string, desiredCN string, desiredSANSs []string) (bool, error) {
	cert, err := certificate.ParseCertificate(certData)
	if err != nil {
		return false, fmt.Errorf("parse certificate: %w", err)
	}

	if time.Until(cert.NotAfter) < certOutdatedDuration {
		return true, nil
	}

	if desiredCN != "" && cert.Subject.CommonName != desiredCN {
		return true, nil
	}

	// Legacy certificates issued before the Subject DN differentiation rule
	// have Subject == Issuer and are rejected by strict validators. Force a
	// re-issue so the new code path can produce a compliant certificate.
	if cert.Subject.String() == cert.Issuer.String() {
		return true, nil
	}

	// Legacy "requestheader-client" usage produced certificates without any
	// ExtendedKeyUsage. Detect them and force re-issue.
	if !hasAnyExtendedKeyUsage(cert) {
		return true, nil
	}

	var dnsNames, ipAddrs []string
	for _, san := range desiredSANSs {
		switch {
		case net.IsIPv4String(san), net.IsIPv6String(san):
			ipAddrs = append(ipAddrs, san)
		default:
			dnsNames = append(dnsNames, san)
		}
	}

	if !arraysAreEqual(dnsNames, cert.DNSNames) {
		return true, nil
	}

	if len(ipAddrs) > 0 {
		certIPs := make([]string, 0, len(cert.IPAddresses))
		for _, cip := range cert.IPAddresses {
			certIPs = append(certIPs, cip.String())
		}
		if !arraysAreEqual(ipAddrs, certIPs) {
			return true, nil
		}
	}

	return false, nil
}

// hasAnyExtendedKeyUsage reports whether the certificate carries an
// ExtendedKeyUsage extension. Either the standard ExtKeyUsage slice or the
// raw UnknownExtKeyUsage slice (custom OIDs) is enough.
func hasAnyExtendedKeyUsage(cert *x509.Certificate) bool {
	return len(cert.ExtKeyUsage) > 0 || len(cert.UnknownExtKeyUsage) > 0
}

func isOutdatedCA(ca string) (bool, error) {
	// Issue a new certificate if there is no CA in the secret.
	// Without CA it is not possible to validate the certificate.
	if len(ca) == 0 {
		return true, nil
	}

	cert, err := certificate.ParseCertificate(ca)
	if err != nil {
		return false, fmt.Errorf("parse certificate: %w", err)
	}

	if time.Until(cert.NotAfter) < certOutdatedDuration {
		return true, nil
	}

	return false, nil
}

func generateNewSelfSignedTLS(input *go_hook.HookInput, conf GenSelfSignedTLSHookConf, sans, usages []string) (certificate.Certificate, error) {
	ca, err := certificate.GenerateCA(input.Logger,
		conf.CN,
		certificate.WithKeyAlgo(keyAlgorithm),
		certificate.WithKeySize(keySize),
		certificate.WithCAExpiry(caExpiryDurationStr),
		// O=Deckhouse, OU=<module> on the CA → leaf's CN-only Subject
		// differs from the CA's Subject. Do NOT replicate WithNames on
		// GenerateSelfSignedCert below, otherwise Subject(leaf) ==
		// Subject(CA) and the depth-0 self-signed collision returns.
		certificate.WithNames(csr.Name{O: caOrganization, OU: conf.caOU()}),
	)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("generate ca: %w", err)
	}

	cert, err := certificate.GenerateSelfSignedCert(input.Logger,
		conf.CN,
		ca,
		certificate.WithSANs(sans...),
		certificate.WithKeyAlgo(keyAlgorithm),
		certificate.WithKeySize(keySize),
		certificate.WithSigningDefaultExpiry(certExpiryDuration),
		certificate.WithSigningDefaultUsage(usages),
	)
	if err != nil {
		return certificate.Certificate{}, fmt.Errorf("generate self signed cert: %w", err)
	}
	return cert, nil
}

// SANsGenerator function for generating sans
type SANsGenerator func(_ context.Context, input *go_hook.HookInput) []string
