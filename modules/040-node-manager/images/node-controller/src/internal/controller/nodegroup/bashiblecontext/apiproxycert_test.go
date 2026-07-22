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

package bashiblecontext

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

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func testCertificatePEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: certCommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}, &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: certCommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestEnsureCertificateKeepsFreshSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: apiProxyCertSecretName, Namespace: kubeSystemNS},
		Data: map[string][]byte{
			"crt": testCertificatePEM(t, time.Now().Add(certOutdatedDuration+time.Hour)),
			"key": []byte("old-key"),
		},
	}
	c := &Controller{clientset: fake.NewSimpleClientset(secret)}

	require.NoError(t, c.ensureCertificate(context.Background(), logr.Discard()))

	csrs, err := c.clientset.CertificatesV1().CertificateSigningRequests().List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, csrs.Items)
}

func TestWriteCertSecretCreatesAndUpdates(t *testing.T) {
	c := &Controller{clientset: fake.NewSimpleClientset()}

	require.NoError(t, c.writeCertSecret(context.Background(), []byte("crt-1"), []byte("key-1")))
	got, err := c.clientset.CoreV1().Secrets(kubeSystemNS).Get(context.Background(), apiProxyCertSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "deckhouse", got.Labels["heritage"])
	assert.Equal(t, "node-manager", got.Labels["module"])
	assert.Equal(t, []byte("crt-1"), got.Data["crt"])
	assert.Equal(t, []byte("key-1"), got.Data["key"])

	got.Labels["keep"] = "me"
	_, err = c.clientset.CoreV1().Secrets(kubeSystemNS).Update(context.Background(), got, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.NoError(t, c.writeCertSecret(context.Background(), []byte("crt-2"), []byte("key-2")))
	got, err = c.clientset.CoreV1().Secrets(kubeSystemNS).Get(context.Background(), apiProxyCertSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "me", got.Labels["keep"])
	assert.Equal(t, []byte("crt-2"), got.Data["crt"])
	assert.Equal(t, []byte("key-2"), got.Data["key"])
}
