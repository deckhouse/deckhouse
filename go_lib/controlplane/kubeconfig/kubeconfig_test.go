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
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

type mockCertProvider struct {
	caCert   *x509.Certificate
	caKey    crypto.Signer
	notAfter time.Time
}

func (m *mockCertProvider) NotAfter() time.Time       { return m.notAfter }
func (m *mockCertProvider) CACert() *x509.Certificate { return m.caCert }
func (m *mockCertProvider) CAKey() crypto.Signer      { return m.caKey }

func createCACert(t *testing.T) (*x509.Certificate, crypto.Signer) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-ca",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
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

func TestCreateKubeConfigFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kubeconfig-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	caCert, caKey := createCACert(t)
	certProvider := &mockCertProvider{
		caCert:   caCert,
		caKey:    caKey,
		notAfter: time.Now().Add(365 * 24 * time.Hour),
	}

	opt := &options{
		OutDir:               tmpDir,
		ClusterName:          "test-cluster",
		ControlPlaneEndpoint: "https://1.2.3.4:6443",
		LocalAPIEndpoint:     "https://127.0.0.1:6443",
		CertProvider:         certProvider,
		EncryptionAlgorithm:  constants.EncryptionAlgorithmRSA2048,
		NodeName:             "test-node",
	}

	tests := []struct {
		name    string
		file    File
		wantErr bool
	}{
		{
			name:    "Admin kubeconfig",
			file:    Admin,
			wantErr: false,
		},
		{
			name:    "SuperAdmin kubeconfig",
			file:    SuperAdmin,
			wantErr: false,
		},
		{
			name:    "ControllerManager kubeconfig",
			file:    ControllerManager,
			wantErr: false,
		},
		{
			name:    "Scheduler kubeconfig",
			file:    Scheduler,
			wantErr: false,
		},
		{
			name:    "Kubelet kubeconfig",
			file:    Kubelet,
			wantErr: false,
		},
		{
			name:    "Unsupported kind",
			file:    File("unknown.conf"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rep KubeconfigApplyReport
			err := createKubeConfigFile(tt.file, opt, &rep)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, rep.Entries)
				return
			}
			require.NoError(t, err)
			require.Len(t, rep.Entries, 1)
			assert.Equal(t, tt.file, rep.Entries[0].File)
			assert.Equal(t, KubeconfigActionWrittenCreated, rep.Entries[0].Action)

			// Verify file exists
			filePath := filepath.Join(tmpDir, string(tt.file))
			assert.FileExists(t, filePath)

			rep = KubeconfigApplyReport{}
			err = createKubeConfigFile(tt.file, opt, &rep)
			require.NoError(t, err)
			require.Len(t, rep.Entries, 1)
			assert.Equal(t, KubeconfigActionUnchanged, rep.Entries[0].Action)
		})
	}
}

func TestCreateKubeConfigFile_Recreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kubeconfig-recreation-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	caCert, caKey := createCACert(t)
	certProvider := &mockCertProvider{
		caCert:   caCert,
		caKey:    caKey,
		notAfter: time.Now().Add(365 * 24 * time.Hour),
	}

	baseOpt := &options{
		OutDir:               tmpDir,
		ClusterName:          "test-cluster",
		ControlPlaneEndpoint: "https://1.2.3.4:6443",
		LocalAPIEndpoint:     "https://127.0.0.1:6443",
		CertProvider:         certProvider,
		EncryptionAlgorithm:  constants.EncryptionAlgorithmRSA2048,
		NodeName:             "test-node",
	}

	var (
		expiredCertProvider *mockCertProvider
		expiredOpt          options
		stat1               os.FileInfo
	)

	t.Run("Recreate when certificate expires soon", func(t *testing.T) {
		file := Admin
		filePath := filepath.Join(tmpDir, string(file))

		// a) Create file with soon-to-expire cert
		expiredCertProvider = &mockCertProvider{
			caCert:   caCert,
			caKey:    caKey,
			notAfter: time.Now().Add(5 * 24 * time.Hour), // < 30 days
		}
		expiredOpt = *baseOpt
		expiredOpt.CertProvider = expiredCertProvider

		var rep KubeconfigApplyReport
		err := createKubeConfigFile(file, &expiredOpt, &rep)
		require.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenCreated, rep.Entries[0].Action)

		// Wait to ensure file is written and modTime is set
		stat1, err = os.Stat(filePath)
		require.NoError(t, err)

		// Verification: check if certificateExpiresSoon works as expected for the file we just created
		{
			cfg, err := clientcmd.LoadFromFile(filePath)
			require.NoError(t, err)
			certData := cfg.AuthInfos[cfg.Contexts[cfg.CurrentContext].AuthInfo].ClientCertificateData
			block, _ := pem.Decode(certData)
			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			// Debug info
			t.Logf("Cert NotAfter: %v, Now: %v, Threshold: %v", cert.NotAfter, time.Now(), 30*24*time.Hour)

			if !pkiutil.CertificateExpiresSoon(cert, 30*24*time.Hour) {
				t.Errorf("Initial certificate SHOULD be expiring soon. NotAfter: %v", cert.NotAfter)
			}
		}

		// b) Call with long-lived cert provider. It should detect that the CURRENT file has an expiring cert and recreate it.
		time.Sleep(100 * time.Millisecond)
		rep = KubeconfigApplyReport{}
		err = createKubeConfigFile(file, baseOpt, &rep)
		assert.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenRegenerated, rep.Entries[0].Action)

		stat2, _ := os.Stat(filePath)
		assert.True(t, stat2.ModTime().After(stat1.ModTime()), "File should have been recreated because the existing cert was expiring")

		// Verify that the NEW cert in the file is NOT expiring soon
		cfg, err := clientcmd.LoadFromFile(filePath)
		require.NoError(t, err)
		certData := cfg.AuthInfos[cfg.Contexts[cfg.CurrentContext].AuthInfo].ClientCertificateData
		block, _ := pem.Decode(certData)
		cert, _ := x509.ParseCertificate(block.Bytes)
		assert.False(t, pkiutil.CertificateExpiresSoon(cert, 30*24*time.Hour), "New certificate should not be expiring soon")

		os.Remove(filePath)
		require.NoError(t, err)
	})

	t.Run("Recreate when API server address changes", func(t *testing.T) {
		file := Admin
		filePath := filepath.Join(tmpDir, string(file))

		// 1. Create initial
		var rep KubeconfigApplyReport
		err := createKubeConfigFile(file, baseOpt, &rep)
		require.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenCreated, rep.Entries[0].Action)
		stat1, _ := os.Stat(filePath)

		// 2. Change API server
		newAddrOpt := *baseOpt
		newAddrOpt.ControlPlaneEndpoint = "https://9.9.9.9:6443"

		// 3. Recreate
		time.Sleep(100 * time.Millisecond)
		rep = KubeconfigApplyReport{}
		err = createKubeConfigFile(file, &newAddrOpt, &rep)
		assert.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenRegenerated, rep.Entries[0].Action)

		stat2, _ := os.Stat(filePath)
		assert.True(t, stat2.ModTime().After(stat1.ModTime()), "File should have been recreated due to API server change")

		// Verify content
		cfg, err := clientcmd.LoadFromFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "https://9.9.9.9:6443", cfg.Clusters[cfg.Contexts[cfg.CurrentContext].Cluster].Server)

		os.Remove(filePath)
		require.NoError(t, err)
	})

	t.Run("Recreate when CA certificate changes", func(t *testing.T) {
		file := Admin
		filePath := filepath.Join(tmpDir, string(file))

		// 1. Create initial
		var rep KubeconfigApplyReport
		err := createKubeConfigFile(file, baseOpt, &rep)
		require.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenCreated, rep.Entries[0].Action)
		stat1, _ := os.Stat(filePath)

		// 2. Change CA
		newCACert, newCAKey := createCACert(t)
		newCAProvider := &mockCertProvider{
			caCert:   newCACert,
			caKey:    newCAKey,
			notAfter: time.Now().Add(365 * 24 * time.Hour),
		}
		newCAOpt := *baseOpt
		newCAOpt.CertProvider = newCAProvider

		// 3. Recreate
		time.Sleep(100 * time.Millisecond)
		rep = KubeconfigApplyReport{}
		err = createKubeConfigFile(file, &newCAOpt, &rep)
		assert.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenRegenerated, rep.Entries[0].Action)

		stat2, _ := os.Stat(filePath)
		assert.True(t, stat2.ModTime().After(stat1.ModTime()), "File should have been recreated due to CA change")

		os.Remove(filePath)
		require.NoError(t, err)
	})

	t.Run("Recreate when file is corrupted", func(t *testing.T) {
		file := Admin
		filePath := filepath.Join(tmpDir, string(file))

		// 1. Create initial
		var rep KubeconfigApplyReport
		err := createKubeConfigFile(file, baseOpt, &rep)
		require.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenCreated, rep.Entries[0].Action)

		// 2. Corrupt file
		err = os.WriteFile(filePath, []byte("not a kubeconfig"), 0644)
		require.NoError(t, err)
		stat1, _ := os.Stat(filePath)

		// 3. Recreate
		time.Sleep(100 * time.Millisecond)
		rep = KubeconfigApplyReport{}
		err = createKubeConfigFile(file, baseOpt, &rep)
		assert.NoError(t, err)
		require.Len(t, rep.Entries, 1)
		assert.Equal(t, KubeconfigActionWrittenRegenerated, rep.Entries[0].Action)

		stat2, _ := os.Stat(filePath)
		assert.True(t, stat2.ModTime().After(stat1.ModTime()), "File should have been recreated due to corruption")

		os.Remove(filePath)
		require.NoError(t, err)
	})
}
