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
	"net"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
)

// makeTestConfig returns a minimal valid config pointing to dir.
// It uses pki2-internal symbols (newConfig, WithPKIDir) and therefore cannot
// be placed in a shared test package.
func makeTestConfig(t *testing.T, dir string) config {
	t.Helper()
	cfg, err := newConfig(
		"test-node",
		"cluster.local",
		net.ParseIP("10.0.0.1"),
		"10.96.0.0/12",
		WithPKIDir(dir),
	)
	require.NoError(t, err)
	return *cfg
}

// makeTestCACert returns a valid long-lived self-signed CA certificate and its private key.
func makeTestCACert(t *testing.T, commonName string) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSelfSignedCACert(pkiutil.CertConfig{
		Config:                    certutil.Config{CommonName: commonName},
		CertificateValidityPeriod: constants.CACertificateValidityPeriod,
	}, key)
	require.NoError(t, err)
	return cert, key
}

// makeExpiringSoonCACert returns a self-signed CA certificate that expires within 30 days
// (NotAfter = now+24h), causing validateCert-style checks to treat it as invalid.
func makeExpiringSoonCACert(t *testing.T, commonName string) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSelfSignedCACert(pkiutil.CertConfig{
		Config:                    certutil.Config{CommonName: commonName},
		CertificateValidityPeriod: constants.CACertificateValidityPeriod,
		NotAfter:                  time.Now().Add(24 * time.Hour),
	}, key)
	require.NoError(t, err)
	return cert, key
}

// makeExpiringSoonLeafCert returns a leaf certificate signed by caCert/caKey
// that expires within 30 days (NotAfter = now+24h).
func makeExpiringSoonLeafCert(t *testing.T, commonName string, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSignedCert(pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: commonName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
		NotAfter:                  time.Now().Add(24 * time.Hour),
	}, key, caCert, caKey)
	require.NoError(t, err)
	return cert, key
}

// makeCertForValidation creates a real x509.Certificate from the given certutil.Config.
// Useful for testing certificate validation logic without writing files to disk.
func makeCertForValidation(t *testing.T, cfg certutil.Config) *x509.Certificate {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSelfSignedCACert(pkiutil.CertConfig{
		Config:                    cfg,
		CertificateValidityPeriod: constants.CACertificateValidityPeriod,
	}, key)
	require.NoError(t, err)
	return cert
}
