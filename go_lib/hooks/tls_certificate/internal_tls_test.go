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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/require"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func testGetClusterDomainValues(t *testing.T, domain string) sdkpkg.PatchableValuesCollector {
	patchableValues, err := sdkpatchablevalues.NewPatchableValues(map[string]interface{}{
		"global": map[string]interface{}{
			"discovery": map[string]interface{}{
				"clusterDomain": domain,
			},
		},
	})
	require.NoError(t, err)
	return patchableValues
}

func TestDefaultSANs(t *testing.T) {
	orig := []string{
		"conversion-webhook-handler.d8-system.svc",
		ClusterDomainSAN("conversion-webhook-handler.d8-system.svc"),
	}
	f := DefaultSANs(orig)

	patchableValues1 := testGetClusterDomainValues(t, "example1.com")
	res1 := f(context.TODO(), &go_hook.HookInput{Values: patchableValues1})

	require.Equal(t, []string{
		"conversion-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc.example1.com",
	}, res1)

	patchableValues2 := testGetClusterDomainValues(t, "example2.com")
	res2 := f(context.TODO(), &go_hook.HookInput{Values: patchableValues2})

	require.Equal(t, []string{
		"conversion-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc.example2.com",
	}, res2)
}

func TestDefaultUsagesContainsServerAuth(t *testing.T) {
	require.Contains(t, defaultUsages, "server auth",
		`server certificates must carry "server auth"; legacy "requestheader-client" is invalid`)
	require.Contains(t, defaultUsages, "signing")
	require.Contains(t, defaultUsages, "key encipherment")
	require.NotContains(t, defaultUsages, "requestheader-client",
		`"requestheader-client" is not a valid cfssl usage and is silently dropped`)
}

func TestCAOrganizationalUnitDerivation(t *testing.T) {
	cases := []struct {
		name string
		conf GenSelfSignedTLSHookConf
		want string
	}{
		{
			name: "explicit OU wins",
			conf: GenSelfSignedTLSHookConf{
				CN:                   "webhook.d8-system.svc",
				Namespace:            "d8-system",
				CAOrganizationalUnit: "my-module",
			},
			want: "my-module",
		},
		{
			name: "namespace with d8- prefix is stripped",
			conf: GenSelfSignedTLSHookConf{
				CN:        "webhook.d8-foo.svc",
				Namespace: "d8-foo",
			},
			want: "foo",
		},
		{
			name: "namespace without d8- prefix falls through to CN",
			conf: GenSelfSignedTLSHookConf{
				CN:        "webhook",
				Namespace: "kube-system",
			},
			want: "webhook",
		},
		{
			name: "empty namespace falls back to CN",
			conf: GenSelfSignedTLSHookConf{
				CN: "lonely-cn",
			},
			want: "lonely-cn",
		},
		{
			name: "bare d8- prefix falls back to CN",
			conf: GenSelfSignedTLSHookConf{
				CN:        "fallback-cn",
				Namespace: "d8-",
			},
			want: "fallback-cn",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.conf.caOU())
		})
	}
}

// TestGenerateNewSelfSignedTLS_RFC5280 enforces the rules from the
// Deckhouse self-signed certificate spec on certificates produced by the
// central hook. The bulk of the contract is checked by the shared helper
// AssertCertBundleValid (see test_assertions.go); this test only adds
// hook-specific assertions that the helper intentionally leaves open
// (configurable values like the exact OU).
func TestGenerateNewSelfSignedTLS_RFC5280(t *testing.T) {
	logger := log.NewNop()
	input := &go_hook.HookInput{Logger: logger}

	conf := GenSelfSignedTLSHookConf{
		CN:                   "webhook-handler.d8-system.svc",
		Namespace:            "d8-system",
		CAOrganizationalUnit: "deckhouse",
	}
	sans := []string{
		"webhook-handler.d8-system",
		"webhook-handler.d8-system.svc",
		"webhook-handler.d8-system.svc.cluster.local",
	}

	bundle, err := generateNewSelfSignedTLS(input, conf, sans, defaultUsages)
	require.NoError(t, err)

	// Full spec checklist: openssl-equivalent verify, Subject DN sanity,
	// SAN/EKU, NotAfter window, X509KeyPair.
	AssertCertBundleValid(t, bundle.CA, bundle.Cert, bundle.Key, conf.CN, sans)

	// Hook-specific: the configured OU must end up on the CA's Subject DN.
	caCert, err := certificate.ParseCertificate(bundle.CA)
	require.NoError(t, err)
	require.Contains(t, caCert.Subject.OrganizationalUnit, "deckhouse",
		"CA Subject must contain the configured OU")
}

func TestGenerateNewSelfSignedTLS_FallbacksFromNamespace(t *testing.T) {
	logger := log.NewNop()
	input := &go_hook.HookInput{Logger: logger}

	conf := GenSelfSignedTLSHookConf{
		CN:        "capi-controller-manager-webhook",
		Namespace: "d8-cloud-instance-manager",
	}
	sans := []string{"capi-webhook-service.d8-cloud-instance-manager.svc"}
	bundle, err := generateNewSelfSignedTLS(input, conf, sans, defaultUsages)
	require.NoError(t, err)

	AssertCertBundleValid(t, bundle.CA, bundle.Cert, bundle.Key, conf.CN, sans)

	caCert, err := certificate.ParseCertificate(bundle.CA)
	require.NoError(t, err)
	require.Contains(t, caCert.Subject.OrganizationalUnit, "cloud-instance-manager",
		"OU must default to the namespace with d8- prefix stripped")
}

func TestIsIrrelevantCert_CNDrift(t *testing.T) {
	logger := log.NewNop()
	input := &go_hook.HookInput{Logger: logger}

	conf := GenSelfSignedTLSHookConf{
		CN:                   "old-cn.d8-system.svc",
		Namespace:            "d8-system",
		CAOrganizationalUnit: "deckhouse",
	}
	sans := []string{"old-cn.d8-system.svc"}
	bundle, err := generateNewSelfSignedTLS(input, conf, sans, defaultUsages)
	require.NoError(t, err)

	stale, err := isIrrelevantCert(bundle.Cert, "old-cn.d8-system.svc", sans)
	require.NoError(t, err)
	require.False(t, stale, "fresh cert with matching CN+SANs must not be marked irrelevant")

	stale, err = isIrrelevantCert(bundle.Cert, "new-cn.d8-system.svc", sans)
	require.NoError(t, err)
	require.True(t, stale, "CN drift must trigger re-issue")
}

func TestIsIrrelevantCert_IPv6SAN(t *testing.T) {
	logger := log.NewNop()
	input := &go_hook.HookInput{Logger: logger}

	conf := GenSelfSignedTLSHookConf{
		CN:                   "ipv6-cert",
		Namespace:            "d8-system",
		CAOrganizationalUnit: "deckhouse",
	}
	sans := []string{"localhost", "::1"}

	bundle, err := generateNewSelfSignedTLS(input, conf, sans, defaultUsages)
	require.NoError(t, err)

	leafCert, err := certificate.ParseCertificate(bundle.Cert)
	require.NoError(t, err)

	require.NotContains(t, leafCert.DNSNames, "::1",
		"IPv6 address must not leak into DNSNames")

	stale, err := isIrrelevantCert(bundle.Cert, conf.CN, sans)
	require.NoError(t, err)
	require.False(t, stale,
		"isIrrelevantCert must classify IPv6 SANs as IP addresses, not DNS names")
}

func TestIsIrrelevantCert_LegacySubjectEqualsIssuer(t *testing.T) {
	// Mint a leaf whose Subject DN exactly matches its Issuer DN. This is
	// the legacy depth-0 collision the spec addresses; isIrrelevantCert must
	// classify such a certificate as stale.
	certPEM := mintLegacyTestCert(t, legacyTestCertOptions{
		subjectCN: "legacy-collision",
		issuerCN:  "legacy-collision",
		dnsNames:  []string{"legacy-collision"},
		ekuServer: true,
	})

	stale, err := isIrrelevantCert(certPEM, "legacy-collision", []string{"legacy-collision"})
	require.NoError(t, err)
	require.True(t, stale,
		"a leaf with Subject == Issuer must be re-issued (depth-0 self-signed collision)")
}

func TestIsIrrelevantCert_NoExtendedKeyUsage(t *testing.T) {
	certPEM := mintLegacyTestCert(t, legacyTestCertOptions{
		subjectCN: "legacy-no-eku-leaf",
		issuerCN:  "legacy-no-eku-ca",
		dnsNames:  []string{"legacy-no-eku-leaf"},
		ekuServer: false,
	})

	cert, err := certificate.ParseCertificate(certPEM)
	require.NoError(t, err)
	require.False(t, hasAnyExtendedKeyUsage(cert),
		"hand-rolled fixture must have empty ExtendedKeyUsage")

	stale, err := isIrrelevantCert(certPEM, "legacy-no-eku-leaf", []string{"legacy-no-eku-leaf"})
	require.NoError(t, err)
	require.True(t, stale,
		"a leaf without ExtendedKeyUsage must be re-issued")
}

type legacyTestCertOptions struct {
	subjectCN string
	issuerCN  string
	dnsNames  []string
	ekuServer bool
}

// mintLegacyTestCert produces a self-signed test certificate where the
// Subject DN and Issuer DN are controlled independently. This lets us
// reproduce the broken Subject == Issuer leaf and the empty-EKU leaf
// produced by the legacy code paths without shipping pre-baked PEM blobs.
func mintLegacyTestCert(t *testing.T, opts legacyTestCertOptions) string {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: opts.subjectCN},
		Issuer:       pkix.Name{CommonName: opts.issuerCN},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:     opts.dnsNames,
	}
	if opts.ekuServer {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
