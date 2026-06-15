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

package kubeconfig

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/util/keyutil"
)

func writePKI(t *testing.T, pkiDir string, caCert *x509.Certificate, caKey crypto.Signer) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(pkiDir, "ca.crt"),
		pkiutil.EncodeCertificate(caCert),
		0o600,
	))
	if caKey != nil {
		keyPEM, err := keyutil.MarshalPrivateKeyToPEM(caKey)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(pkiDir, "ca.key"), keyPEM, 0o600))
	}
}

func createExpiredCACert(t *testing.T) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "expired-ca"},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(-1 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)
	return cert, key
}

func TestRenewClientCert_HappyPath(t *testing.T) {
	dir := t.TempDir()
	pkiDir := t.TempDir()

	caCert, caKey := createCACert(t)

	opt := makeExpirationTestOptions(t, dir)
	var rep KubeconfigApplyReport
	require.NoError(t, createKubeConfigFile(Admin, opt, &rep))

	writePKI(t, pkiDir, caCert, caKey)

	origCert, err := loadClientCertificate(filepath.Join(dir, string(Admin)))
	require.NoError(t, err)

	require.NoError(t, RenewClientCert(Admin,
		WithRenewKubeconfigDir(dir),
		WithRenewPKIDir(pkiDir),
	))

	newCert, err := loadClientCertificate(filepath.Join(dir, string(Admin)))
	require.NoError(t, err)

	assert.Equal(t, origCert.Subject.CommonName, newCert.Subject.CommonName)
	assert.Equal(t, origCert.Subject.Organization, newCert.Subject.Organization)
	assert.Equal(t, origCert.ExtKeyUsage, newCert.ExtKeyUsage)
	assert.True(t, newCert.NotAfter.After(time.Now()))
}

func TestRenewClientCert_DryRun(t *testing.T) {
	dir := t.TempDir()
	pkiDir := t.TempDir()

	caCert, caKey := createCACert(t)

	opt := makeExpirationTestOptions(t, dir)
	var rep KubeconfigApplyReport
	require.NoError(t, createKubeConfigFile(Admin, opt, &rep))

	writePKI(t, pkiDir, caCert, caKey)

	statBefore, err := os.Stat(filepath.Join(dir, string(Admin)))
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	require.NoError(t, RenewClientCert(Admin,
		WithRenewKubeconfigDir(dir),
		WithRenewPKIDir(pkiDir),
		WithDryRun(),
	))

	statAfter, err := os.Stat(filepath.Join(dir, string(Admin)))
	require.NoError(t, err)
	assert.Equal(t, statBefore.ModTime(), statAfter.ModTime(), "dry-run must not modify the file on disk")
}

func TestRenewClientCert_ErrorSentinels(t *testing.T) {
	tests := []struct {
		name      string
		setupPKI  func(t *testing.T, dir, pkiDir string)
		setupKube func(t *testing.T, dir string)
		wantErr   interface{ Error() string }
	}{
		{
			name: "kubeconfig missing",
			setupPKI: func(t *testing.T, _, pkiDir string) {
				caCert, caKey := createCACert(t)
				writePKI(t, pkiDir, caCert, caKey)
			},
			setupKube: func(_ *testing.T, _ string) {},
			wantErr:   &MissingError{},
		},
		{
			name:     "CA cert missing",
			setupPKI: func(_ *testing.T, _, _ string) {},
			setupKube: func(t *testing.T, dir string) {
				opt := makeExpirationTestOptions(t, dir)
				var rep KubeconfigApplyReport
				require.NoError(t, createKubeConfigFile(Admin, opt, &rep))
			},
			wantErr: &CAMissingError{},
		},
		{
			name: "external CA — key absent",
			setupPKI: func(t *testing.T, _, pkiDir string) {
				caCert, _ := createCACert(t)
				writePKI(t, pkiDir, caCert, nil)
			},
			setupKube: func(t *testing.T, dir string) {
				opt := makeExpirationTestOptions(t, dir)
				var rep KubeconfigApplyReport
				require.NoError(t, createKubeConfigFile(Admin, opt, &rep))
			},
			wantErr: &CAExternalError{},
		},
		{
			name: "CA expired",
			setupPKI: func(t *testing.T, _, pkiDir string) {
				expiredCert, expiredKey := createExpiredCACert(t)
				writePKI(t, pkiDir, expiredCert, expiredKey)
			},
			setupKube: func(t *testing.T, dir string) {
				opt := makeExpirationTestOptions(t, dir)
				var rep KubeconfigApplyReport
				require.NoError(t, createKubeConfigFile(Admin, opt, &rep))
			},
			wantErr: &CAExpiredError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			pkiDir := t.TempDir()

			tt.setupKube(t, dir)
			tt.setupPKI(t, dir, pkiDir)

			err := RenewClientCert(Admin,
				WithRenewKubeconfigDir(dir),
				WithRenewPKIDir(pkiDir),
			)
			require.Error(t, err)
			assert.ErrorAs(t, err, &tt.wantErr)
		})
	}
}
