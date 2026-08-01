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

package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// writePKIDir lays out a complete PKI tree, every file holds its own relative path as content.
// extraFiles are written on top of it, keyed by path relative to the PKI directory.
func writePKIDir(t *testing.T, extraFiles map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "etcd"), 0o700))

	for _, relPath := range requiredPKIFiles {
		writePKIFile(t, dir, relPath, relPath)
	}
	for relPath, content := range extraFiles {
		writePKIFile(t, dir, relPath, content)
	}

	return dir
}

func writePKIFile(t *testing.T, dir, relPath, content string) {
	t.Helper()

	path := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func getPKISecret(t *testing.T, client *fake.Clientset) *corev1.Secret {
	t.Helper()

	secret, err := client.CoreV1().Secrets(pkiSecretNamespace).Get(context.Background(), pkiSecretName, metav1.GetOptions{})
	require.NoError(t, err)

	return secret
}

func TestEnsurePKISecretCreatesRequiredKeys(t *testing.T) {
	client := fake.NewClientset()
	pkiDir := writePKIDir(t, nil)

	require.NoError(t, ensurePKISecret(context.Background(), client, pkiDir))

	secret := getPKISecret(t, client)
	require.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	require.Len(t, secret.Data, len(requiredPKIFiles))
	for key, relPath := range requiredPKIFiles {
		require.Equal(t, relPath, string(secret.Data[key]), "unexpected content of key %s", key)
	}
	require.Equal(t, "etcd/ca.crt", string(secret.Data["etcd-ca.crt"]))
	require.Equal(t, "etcd/ca.key", string(secret.Data["etcd-ca.key"]))
}

func TestEnsurePKISecretWithSignaturePair(t *testing.T) {
	client := fake.NewClientset()
	pkiDir := writePKIDir(t, map[string]string{
		"signature-private.jwk": "private-jwk",
		"signature-public.jwks": "public-jwks",
	})

	require.NoError(t, ensurePKISecret(context.Background(), client, pkiDir))

	secret := getPKISecret(t, client)
	require.Len(t, secret.Data, len(requiredPKIFiles)+2)
	require.Equal(t, "private-jwk", string(secret.Data[signaturePrivateKey]))
	require.Equal(t, "public-jwks", string(secret.Data[signaturePublicKey]))
}

func TestEnsurePKISecretIncompleteSignaturePair(t *testing.T) {
	for _, tc := range []struct {
		name    string
		relPath string
	}{
		{name: "only private", relPath: "signature-private.jwk"},
		{name: "only public", relPath: "signature-public.jwks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset()
			pkiDir := writePKIDir(t, map[string]string{tc.relPath: "content"})

			err := ensurePKISecret(context.Background(), client, pkiDir)
			require.ErrorContains(t, err, "required together")

			_, getErr := client.CoreV1().Secrets(pkiSecretNamespace).Get(context.Background(), pkiSecretName, metav1.GetOptions{})
			require.True(t, apierrors.IsNotFound(getErr), "secret must not be created")
		})
	}
}

func TestEnsurePKISecretMissingRequiredFile(t *testing.T) {
	client := fake.NewClientset()
	pkiDir := writePKIDir(t, nil)
	require.NoError(t, os.Remove(filepath.Join(pkiDir, "etcd", "ca.key")))

	err := ensurePKISecret(context.Background(), client, pkiDir)
	require.ErrorContains(t, err, "read pki file")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsurePKISecretUpdatesChangedData(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: pkiSecretName, Namespace: pkiSecretNamespace},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt":            []byte("stale"),
			signaturePrivateKey: []byte("rotated-by-control-plane-manager"),
		},
	}
	client := fake.NewClientset(existing)
	pkiDir := writePKIDir(t, nil)

	require.NoError(t, ensurePKISecret(context.Background(), client, pkiDir))

	secret := getPKISecret(t, client)
	require.Equal(t, "ca.crt", string(secret.Data["ca.crt"]))
	require.Equal(t, "rotated-by-control-plane-manager", string(secret.Data[signaturePrivateKey]), "unmanaged keys must be preserved")
	require.Len(t, secret.Data, len(requiredPKIFiles)+1)
}

func TestEnsurePKISecretDoesNotUpdateEqualData(t *testing.T) {
	client := fake.NewClientset()
	pkiDir := writePKIDir(t, nil)

	require.NoError(t, ensurePKISecret(context.Background(), client, pkiDir))
	client.ClearActions()
	require.NoError(t, ensurePKISecret(context.Background(), client, pkiDir))

	for _, action := range client.Actions() {
		require.Equal(t, "get", action.GetVerb(), "unexpected write on an up-to-date secret: %v", action)
	}
	require.Len(t, client.Actions(), 1)
	require.IsType(t, k8stesting.GetActionImpl{}, client.Actions()[0])
}
