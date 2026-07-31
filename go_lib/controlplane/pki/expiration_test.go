/*
Copyright 2026 Flant JSC

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

package pki

import (
	"crypto"
	"crypto/x509"
	"path/filepath"
	"sort"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
)

func TestListCertificateExpirations_DefaultInventory(t *testing.T) {
	dir := t.TempDir()

	_, err := CreatePKIBundle(
		"test-node",
		"cluster.local",
		makeTestConfig(t, dir).AdvertiseAddress,
		"10.96.0.0/12",
		WithPKIDir(dir),
	)
	require.NoError(t, err)

	report, err := ListCertificateExpirations(WithCertificatesDir(dir))
	require.NoError(t, err)
	require.Len(t, report.Entries, 10)

	paths := make([]string, 0, len(report.Entries))
	for _, e := range report.Entries {
		require.NoError(t, e.Err, "entry %q should have no error", e.Name)
		paths = append(paths, e.Path)
	}

	assert.True(t, sort.StringsAreSorted(paths))
	assert.NotContains(t, paths, filepath.Join(dir, "sa.crt"))
}

func TestListCertificateExpirations_MissingEntries(t *testing.T) {
	dir := t.TempDir()
	caCert, _ := makeTestCACert(t, "kubernetes")

	require.NoError(t, writeCert(dir, string(CACertBaseName), caCert))

	report, err := ListCertificateExpirations(
		WithCertificatesDir(dir),
		WithRootCertificates(CACertBaseName, CACertBaseName, EtcdCACertBaseName),
	)
	require.NoError(t, err)
	require.Len(t, report.Entries, 2)

	byName := make(map[string]ExpirationEntry, len(report.Entries))
	for _, e := range report.Entries {
		byName[e.Name] = e
	}

	caEntry, ok := byName[string(CACertBaseName)]
	require.True(t, ok)
	require.NoError(t, caEntry.Err)
	assert.True(t, caEntry.IsCA)

	etcdCAEntry, ok := byName[string(EtcdCACertBaseName)]
	require.True(t, ok)
	var missing *MissingError
	require.ErrorAs(t, etcdCAEntry.Err, &missing)
	assert.Equal(t, string(EtcdCACertBaseName), missing.BaseName)
	assert.Equal(t, filepath.Join(dir, "etcd", "ca.crt"), etcdCAEntry.Path)
}

func TestGetCertificateExpiration_NormalizesKnownAndUnknownPaths(t *testing.T) {
	dir := t.TempDir()
	etcdCACert, etcdCAKey := makeTestCACert(t, "etcd-ca")
	etcdServerCert := makeTestLeafCert(t, "etcd-server", etcdCACert, etcdCAKey)
	customCert, _ := makeTestCACert(t, "custom-ca")

	require.NoError(t, writeCert(dir, string(EtcdServerCertBaseName), etcdServerCert))
	require.NoError(t, writeCert(dir, "custom/custom-cert", customCert))

	knownPath := filepath.Join(dir, "etcd", "server.crt")
	knownExpiration, err := GetCertificateExpiration(knownPath)
	require.NoError(t, err)
	assert.Equal(t, string(EtcdServerCertBaseName), knownExpiration.Name)
	assert.Equal(t, EtcdCACertBaseName, knownExpiration.Authority)
	assert.False(t, knownExpiration.IsCA)
	assert.Equal(t, filepath.Clean(knownPath), knownExpiration.Path)

	unknownPath := filepath.Join(dir, "custom", "custom-cert.crt")
	unknownExpiration, err := GetCertificateExpiration(unknownPath)
	require.NoError(t, err)
	assert.Equal(t, "custom-cert", unknownExpiration.Name)
	assert.Empty(t, unknownExpiration.Authority)
	assert.True(t, unknownExpiration.IsCA)
	assert.Equal(t, filepath.Clean(unknownPath), unknownExpiration.Path)
}

func makeTestLeafCert(t *testing.T, commonName string, caCert *x509.Certificate, caKey crypto.Signer) *x509.Certificate {
	t.Helper()

	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSignedCert(pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: commonName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
	}, key, caCert, caKey)
	require.NoError(t, err)

	return cert
}
