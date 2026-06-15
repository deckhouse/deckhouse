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
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
)

func TestLoadClientCertificate(t *testing.T) {
	dir := t.TempDir()
	opt := makeExpirationTestOptions(t, dir)

	var rep KubeconfigApplyReport
	require.NoError(t, createKubeConfigFile(Admin, opt, &rep))

	cert, err := loadClientCertificate(filepath.Join(dir, string(Admin)))
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-admin", cert.Subject.CommonName)
}

func TestListClientCertificateExpirations_DefaultExcludesKubelet(t *testing.T) {
	dir := t.TempDir()
	opt := makeExpirationTestOptions(t, dir)

	for _, file := range []File{SuperAdmin, Admin, Scheduler, ControllerManager, Kubelet} {
		var rep KubeconfigApplyReport
		require.NoError(t, createKubeConfigFile(file, opt, &rep))
	}

	report := ListClientCertificateExpirations(WithKubeconfigDir(dir))

	require.Len(t, report.Entries, 4)
	assert.Equal(t, []File{Admin, ControllerManager, Scheduler, SuperAdmin}, expirationFiles(newExpirationOptions(WithKubeconfigDir(dir))))

	files := make([]File, 0, len(report.Entries))
	for _, e := range report.Entries {
		require.NoError(t, e.Err)
		files = append(files, e.File)
	}

	assert.Equal(t, []File{Admin, ControllerManager, Scheduler, SuperAdmin}, files)
	assert.NotContains(t, files, Kubelet)
}

func TestListClientCertificateExpirations_MissingEntries(t *testing.T) {
	dir := t.TempDir()
	opt := makeExpirationTestOptions(t, dir)

	var rep KubeconfigApplyReport
	require.NoError(t, createKubeConfigFile(Admin, opt, &rep))

	report := ListClientCertificateExpirations(
		WithKubeconfigDir(dir),
		WithFiles(Admin, Admin, Scheduler),
	)
	require.Len(t, report.Entries, 2)

	byFile := make(map[File]KubeconfigExpirationEntry, len(report.Entries))
	for _, e := range report.Entries {
		byFile[e.File] = e
	}

	adminEntry, ok := byFile[Admin]
	require.True(t, ok)
	require.NoError(t, adminEntry.Err)

	schedulerEntry, ok := byFile[Scheduler]
	require.True(t, ok)
	var missing *MissingError
	require.ErrorAs(t, schedulerEntry.Err, &missing)
	assert.Equal(t, Scheduler, missing.File)
	assert.Equal(t, filepath.Join(dir, string(Scheduler)), schedulerEntry.Path)
}

func TestGetClientCertificateExpiration_NormalizesKnownAndUnknownPaths(t *testing.T) {
	dir := t.TempDir()
	opt := makeExpirationTestOptions(t, dir)

	var rep KubeconfigApplyReport
	require.NoError(t, createKubeConfigFile(Scheduler, opt, &rep))

	customConfigPath := filepath.Join(dir, "custom.conf")
	config, err := buildConfig(mustGetFileSpec(t, Admin, opt))
	require.NoError(t, err)
	require.NoError(t, clientcmd.WriteToFile(*config, customConfigPath))

	knownExpiration, err := GetClientCertificateExpiration(filepath.Join(dir, string(Scheduler)))
	require.NoError(t, err)
	assert.Equal(t, Scheduler, knownExpiration.File)

	unknownExpiration, err := GetClientCertificateExpiration(customConfigPath)
	require.NoError(t, err)
	assert.Equal(t, File("custom.conf"), unknownExpiration.File)
	assert.Equal(t, filepath.Clean(customConfigPath), unknownExpiration.Path)
}

func TestLoadClientCertificate_FileReference(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey := createCACert(t)

	clientCertConfig := pkiutil.CertConfig{
		Config:              certutilConfig(t, "test-user"),
		NotAfter:            time.Now().Add(24 * time.Hour),
		EncryptionAlgorithm: constants.EncryptionAlgorithmRSA2048,
	}
	clientCert, _, err := pkiutil.NewCertAndKey(caCert, caKey, clientCertConfig)
	require.NoError(t, err)

	certFile := filepath.Join(dir, "client.crt")
	require.NoError(t, os.WriteFile(certFile, pkiutil.EncodeCertificate(clientCert), 0o600))

	buildFileRefConfig := func(certPath string) *clientcmdapi.Config {
		return &clientcmdapi.Config{
			Clusters: map[string]*clientcmdapi.Cluster{
				"test-cluster": {
					Server:                   "https://1.2.3.4:6443",
					CertificateAuthorityData: pkiutil.EncodeCertificate(caCert),
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				"ctx": {Cluster: "test-cluster", AuthInfo: "test-user"},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"test-user": {ClientCertificate: certPath},
			},
			CurrentContext: "ctx",
		}
	}

	t.Run("absolute path", func(t *testing.T) {
		kubeconfigPath := filepath.Join(dir, "abs.conf")
		require.NoError(t, clientcmd.WriteToFile(*buildFileRefConfig(certFile), kubeconfigPath))

		cert, err := loadClientCertificate(kubeconfigPath)
		require.NoError(t, err)
		assert.Equal(t, "test-user", cert.Subject.CommonName)
	})

	t.Run("relative path resolved against kubeconfig dir", func(t *testing.T) {
		kubeconfigPath := filepath.Join(dir, "rel.conf")
		require.NoError(t, clientcmd.WriteToFile(*buildFileRefConfig("client.crt"), kubeconfigPath))

		cert, err := loadClientCertificate(kubeconfigPath)
		require.NoError(t, err)
		assert.Equal(t, "test-user", cert.Subject.CommonName)
	})
}

func certutilConfig(t *testing.T, commonName string) certutil.Config {
	t.Helper()
	return certutil.Config{
		CommonName: commonName,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func makeExpirationTestOptions(t *testing.T, outDir string) *options {
	t.Helper()

	caCert, caKey := createCACert(t)
	certProvider := &mockCertProvider{
		caCert:   caCert,
		caKey:    caKey,
		notAfter: time.Now().Add(365 * 24 * time.Hour),
	}

	return &options{
		OutDir:               outDir,
		ClusterName:          "test-cluster",
		ControlPlaneEndpoint: "https://1.2.3.4:6443",
		LocalAPIEndpoint:     "https://127.0.0.1:6443",
		CertProvider:         certProvider,
		EncryptionAlgorithm:  constants.EncryptionAlgorithmRSA2048,
		NodeName:             "test-node",
	}
}

func mustGetFileSpec(t *testing.T, file File, opt *options) *fileSpec {
	t.Helper()

	spec, err := getFileSpec(file, opt)
	require.NoError(t, err)

	return spec
}
