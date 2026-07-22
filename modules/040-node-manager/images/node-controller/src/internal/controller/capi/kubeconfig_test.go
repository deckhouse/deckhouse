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

package capi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

func TestBuildKubeconfigYAML(t *testing.T) {
	yaml, err := buildKubeconfigYAML(
		"test-cluster",
		"https://api.example.com:6443",
		[]byte("CA-DATA"),
		[]byte("KEY-DATA"),
		[]byte("CRT-DATA"),
	)
	require.NoError(t, err)

	cfg, err := clientcmd.Load(yaml)
	require.NoError(t, err)

	cluster, ok := cfg.Clusters["test-cluster"]
	require.True(t, ok, "cluster entry must be named after the CAPI cluster")
	assert.Equal(t, "https://api.example.com:6443", cluster.Server)
	assert.Equal(t, []byte("CA-DATA"), cluster.CertificateAuthorityData)

	auth, ok := cfg.AuthInfos["capi-controller-manager"]
	require.True(t, ok, "auth info must be the capi-controller-manager identity")
	assert.Equal(t, []byte("KEY-DATA"), auth.ClientKeyData)
	assert.Equal(t, []byte("CRT-DATA"), auth.ClientCertificateData)

	assert.Equal(t, "capi-controller-manager@test-cluster", cfg.CurrentContext)
	ctx, ok := cfg.Contexts["capi-controller-manager@test-cluster"]
	require.True(t, ok)
	assert.Equal(t, "test-cluster", ctx.Cluster)
	assert.Equal(t, "capi-controller-manager", ctx.AuthInfo)
}

// kubeconfigWithCert builds a kubeconfig whose client certificate expires after validFor.
func kubeconfigWithCert(t *testing.T, validFor time.Duration) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "capi-controller-manager"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(validFor),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	yaml, err := buildKubeconfigYAML("c", "https://api", []byte("ca"), []byte("key"), crtPEM)
	require.NoError(t, err)
	return yaml
}

func TestKubeconfigCertFresh(t *testing.T) {
	t.Run("fresh cert is kept", func(t *testing.T) {
		// 180d cert, well above the 90d renew threshold.
		assert.True(t, kubeconfigCertFresh(kubeconfigWithCert(t, 180*24*time.Hour)))
	})

	t.Run("near-expiry cert is renewed", func(t *testing.T) {
		// 30d left, below the 90d renew threshold.
		assert.False(t, kubeconfigCertFresh(kubeconfigWithCert(t, 30*24*time.Hour)))
	})

	t.Run("empty value is not fresh", func(t *testing.T) {
		assert.False(t, kubeconfigCertFresh(nil))
	})

	t.Run("garbage value is not fresh", func(t *testing.T) {
		assert.False(t, kubeconfigCertFresh([]byte("not-a-kubeconfig")))
	})
}
