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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
)

func makeExpiredCACert(t *testing.T, commonName string) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSelfSignedCACert(pkiutil.CertConfig{
		Config:   certutil.Config{CommonName: commonName},
		NotAfter: time.Now().Add(-1 * time.Hour),
	}, key)
	require.NoError(t, err)
	return cert, key
}

func makeLeafCert(t *testing.T, commonName string, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSignedCert(pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: commonName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
	}, key, caCert, caKey)
	require.NoError(t, err)
	return cert, key
}

func TestRenewCertificate_HappyPath(t *testing.T) {
	dir := t.TempDir()

	caCert, caKey := makeTestCACert(t, "kubernetes")
	require.NoError(t, writeCertAndKey(dir, string(CACertBaseName), caCert, caKey))

	leafCert, leafKey := makeLeafCert(t, "kube-apiserver", caCert, caKey)
	require.NoError(t, writeCertAndKey(dir, string(ApiserverCertBaseName), leafCert, leafKey))

	origCert, err := pkiutil.LoadCert(certPath(dir, string(ApiserverCertBaseName)))
	require.NoError(t, err)

	require.NoError(t, RenewCertificate(ApiserverCertBaseName, WithRenewDir(dir)))

	newCert, err := pkiutil.LoadCert(certPath(dir, string(ApiserverCertBaseName)))
	require.NoError(t, err)

	assert.Equal(t, origCert.Subject.CommonName, newCert.Subject.CommonName)
	assert.Equal(t, origCert.Subject.Organization, newCert.Subject.Organization)
	assert.Equal(t, origCert.ExtKeyUsage, newCert.ExtKeyUsage)
	assert.True(t, newCert.NotAfter.After(time.Now()))
}

func TestRenewCertificate_DryRun(t *testing.T) {
	dir := t.TempDir()

	caCert, caKey := makeTestCACert(t, "kubernetes")
	require.NoError(t, writeCertAndKey(dir, string(CACertBaseName), caCert, caKey))

	leafCert, leafKey := makeLeafCert(t, "kube-apiserver", caCert, caKey)
	require.NoError(t, writeCertAndKey(dir, string(ApiserverCertBaseName), leafCert, leafKey))

	statBefore, err := os.Stat(certPath(dir, string(ApiserverCertBaseName)))
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	require.NoError(t, RenewCertificate(ApiserverCertBaseName, WithRenewDir(dir), WithDryRun()))

	statAfter, err := os.Stat(certPath(dir, string(ApiserverCertBaseName)))
	require.NoError(t, err)
	assert.Equal(t, statBefore.ModTime(), statAfter.ModTime(), "dry-run must not modify the file on disk")
}

func TestRenewCertificate_ErrorSentinels(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		wantErr interface{ Error() string }
	}{
		{
			name: "leaf cert missing",
			setup: func(t *testing.T, dir string) {
				caCert, caKey := makeTestCACert(t, "kubernetes")
				require.NoError(t, writeCertAndKey(dir, string(CACertBaseName), caCert, caKey))
			},
			wantErr: &MissingError{},
		},
		{
			name: "CA cert missing",
			setup: func(t *testing.T, dir string) {
				caCert, caKey := makeTestCACert(t, "kubernetes")
				leafCert, leafKey := makeLeafCert(t, "kube-apiserver", caCert, caKey)
				require.NoError(t, writeCertAndKey(dir, string(ApiserverCertBaseName), leafCert, leafKey))
			},
			wantErr: &CAMissingError{},
		},
		{
			name: "external CA — key absent",
			setup: func(t *testing.T, dir string) {
				caCert, caKey := makeTestCACert(t, "kubernetes")
				leafCert, leafKey := makeLeafCert(t, "kube-apiserver", caCert, caKey)
				require.NoError(t, writeCertAndKey(dir, string(ApiserverCertBaseName), leafCert, leafKey))
				// Write only the CA cert, not the key.
				require.NoError(t, writeCert(dir, string(CACertBaseName), caCert))
			},
			wantErr: &CAExternalError{},
		},
		{
			name: "CA expired",
			setup: func(t *testing.T, dir string) {
				expiredCA, expiredKey := makeExpiredCACert(t, "kubernetes")
				freshCA, freshKey := makeTestCACert(t, "kubernetes")
				leafCert, leafKey := makeLeafCert(t, "kube-apiserver", freshCA, freshKey)
				require.NoError(t, writeCertAndKey(dir, string(ApiserverCertBaseName), leafCert, leafKey))
				require.NoError(t, writeCertAndKey(dir, string(CACertBaseName), expiredCA, expiredKey))
			},
			wantErr: &CAExpiredError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			err := RenewCertificate(ApiserverCertBaseName, WithRenewDir(dir))
			require.Error(t, err)
			assert.ErrorAs(t, err, &tt.wantErr)
		})
	}
}

func TestRenewCertificates_AllDefaultLeafs(t *testing.T) {
	dir := t.TempDir()

	cas := map[RootCertBaseName]struct {
		cert *x509.Certificate
		key  crypto.Signer
	}{}
	for _, caName := range []RootCertBaseName{CACertBaseName, FrontProxyCACertBaseName, EtcdCACertBaseName} {
		cert, key := makeTestCACert(t, string(caName))
		require.NoError(t, writeCertAndKey(dir, string(caName), cert, key))
		cas[caName] = struct {
			cert *x509.Certificate
			key  crypto.Signer
		}{cert, key}
	}

	for _, info := range defaultLeafCertificates() {
		caName, ok := caForLeaf(info.Name)
		require.True(t, ok)
		ca := cas[caName]
		leafCert, leafKey := makeLeafCert(t, string(info.Name), ca.cert, ca.key)
		require.NoError(t, writeCertAndKey(dir, string(info.Name), leafCert, leafKey))
	}

	report := RenewCertificates(WithRenewDir(dir))

	require.Len(t, report.Entries, len(defaultLeafCertificates()))
	for _, entry := range report.Entries {
		assert.NoError(t, entry.Err, "cert %s should renew without error", entry.Name)
	}
}

func TestRenewCertificate_UnknownLeaf(t *testing.T) {
	err := RenewCertificate(LeafCertBaseName("not-a-real-cert"), WithRenewDir(t.TempDir()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown leaf certificate")
}

func TestRenewCertificates_WithLeafsSubset(t *testing.T) {
	dir := t.TempDir()

	caCert, caKey := makeTestCACert(t, "kubernetes")
	require.NoError(t, writeCertAndKey(dir, string(CACertBaseName), caCert, caKey))

	for _, name := range []LeafCertBaseName{ApiserverCertBaseName, ApiserverKubeletClientCertBaseName} {
		leafCert, leafKey := makeLeafCert(t, string(name), caCert, caKey)
		require.NoError(t, writeCertAndKey(dir, string(name), leafCert, leafKey))
	}

	statBefore := map[LeafCertBaseName]time.Time{}
	for _, name := range []LeafCertBaseName{ApiserverCertBaseName, ApiserverKubeletClientCertBaseName} {
		info, err := os.Stat(certPath(dir, string(name)))
		require.NoError(t, err)
		statBefore[name] = info.ModTime()
	}

	time.Sleep(50 * time.Millisecond)

	report := RenewCertificates(
		WithRenewDir(dir),
		WithRenewLeafs(ApiserverCertBaseName),
	)

	require.Len(t, report.Entries, 1)
	assert.Equal(t, ApiserverCertBaseName, report.Entries[0].Name)
	assert.NoError(t, report.Entries[0].Err)

	info, err := os.Stat(certPath(dir, string(ApiserverCertBaseName)))
	require.NoError(t, err)
	assert.True(t, info.ModTime().After(statBefore[ApiserverCertBaseName]), "apiserver cert should have been renewed")

	info, err = os.Stat(filepath.Join(dir, string(ApiserverKubeletClientCertBaseName)+".crt"))
	require.NoError(t, err)
	assert.Equal(t, statBefore[ApiserverKubeletClientCertBaseName], info.ModTime(), "apiserver-kubelet-client must not be touched")
}
