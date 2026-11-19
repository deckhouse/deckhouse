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

package controlplaneoperation

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	"github.com/deckhouse/deckhouse/pkg/log"

	cp "control-plane-manager/api/v1alpha1"
)

func TestObserveCertExpirationsForStaticPod_Missing(t *testing.T) {
	for _, file := range []string{"apiserver.crt", "front-proxy-client.crt", "admin.conf", "super-admin.conf"} {
		t.Run(file, func(t *testing.T) {
			pkiDir, configDir := newObservationFixture(t)
			expected := apiserverFixtureDates(t, pkiDir, configDir)
			path := observationPath(pkiDir, configDir, file)
			require.NoError(t, os.Remove(path))
			expected[file] = metav1.NewTime(time.Unix(0, 0).UTC())
			var logs bytes.Buffer
			logger := log.NewLogger(log.WithOutput(&logs), log.WithHandlerType(log.JSONHandlerType))

			state, ok, err := observeCertExpirationsForStaticPod(cp.OperationComponentKubeAPIServer, pkiDir, configDir, logger)

			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expected, state.CertificatesExpirationTime)
			missing := state.CertificatesExpirationTime[file]
			require.False(t, missing.IsZero())
			require.Equal(t, "1970-01-01T00:00:00Z", missing.UTC().Format(time.RFC3339))
			require.Contains(t, logs.String(), path)
			require.Contains(t, logs.String(), "not found")
			require.Contains(t, logs.String(), `"synthetic_expiration":"1970-01-01T00:00:00Z"`)
		})
	}
}

func TestObserveCertExpirationsForStaticPod_ValidCertificates(t *testing.T) {
	pkiDir, configDir := newObservationFixture(t)
	expected := apiserverFixtureDates(t, pkiDir, configDir)

	state, ok, err := observeCertExpirationsForStaticPod(cp.OperationComponentKubeAPIServer, pkiDir, configDir, log.NewNop())

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, expected, state.CertificatesExpirationTime)
}

func TestObserveCertExpirationsForStaticPod_HardErrors(t *testing.T) {
	tests := []struct{ name, broken, missing, contents, want string }{
		{"cert read error", "apiserver.crt", "super-admin.conf", "", "is a directory"},
		{"kubeconfig read error", "admin.conf", "apiserver.crt", "", "is a directory"},
		{"cert parse error", "apiserver.crt", "super-admin.conf", "invalid PEM", "valid RSA or ECDSA certificates"},
		{"kubeconfig parse error", "admin.conf", "apiserver.crt", "contexts: [", "load kubeconfig"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkiDir, configDir := newObservationFixture(t)
			expected := apiserverFixtureDates(t, pkiDir, configDir)
			require.NoError(t, os.Remove(observationPath(pkiDir, configDir, tt.missing)))
			expected[tt.missing] = metav1.NewTime(time.Unix(0, 0).UTC())
			brokenPath := observationPath(pkiDir, configDir, tt.broken)
			require.NoError(t, os.Remove(brokenPath))
			if tt.contents == "" {
				require.NoError(t, os.Mkdir(brokenPath, 0700))
			} else {
				require.NoError(t, os.WriteFile(brokenPath, []byte(tt.contents), 0600))
			}
			delete(expected, tt.broken)

			state, ok, err := observeCertExpirationsForStaticPod(cp.OperationComponentKubeAPIServer, pkiDir, configDir, log.NewNop())

			require.ErrorContains(t, err, tt.want)
			require.True(t, ok)
			require.Equal(t, expected, state.CertificatesExpirationTime)
		})
	}
}

// Generate parsed credentials at runtime, using the same algorithms and paths as renewal.
func newObservationFixture(t *testing.T) (pkiDir, kubeconfigDir string) {
	t.Helper()
	pkiDir, kubeconfigDir = filepath.Join(t.TempDir(), "pki"), t.TempDir()
	_, err := pki.CreatePKIBundle(testNodeName, "cluster.local", net.ParseIP("192.0.2.10"), "10.96.0.0/12",
		pki.WithPKIDir(pkiDir), pki.WithEncryptionAlgorithmType(pkiconstants.EncryptionAlgorithmType("ECDSA-P256")))
	require.NoError(t, err)
	_, err = kubeconfig.CreateKubeconfigFiles([]kubeconfig.File{kubeconfig.Admin, kubeconfig.SuperAdmin},
		kubeconfig.WithCertificatesDir(pkiDir), kubeconfig.WithOutDir(kubeconfigDir), kubeconfig.WithLocalAPIEndpoint("192.0.2.10"),
		kubeconfig.WithEncryptionAlgorithm(pkiconstants.EncryptionAlgorithmType("ECDSA-P256")))
	require.NoError(t, err)
	return
}

// Expected inventory is independent of the observer's dependency registry.
func apiserverFixtureDates(t *testing.T, pkiDir, configDir string) map[string]metav1.Time {
	t.Helper()
	dates := make(map[string]metav1.Time)
	for _, name := range []string{"apiserver.crt", "apiserver-kubelet-client.crt", "front-proxy-client.crt"} {
		cert, err := pki.GetCertificateExpiration(filepath.Join(pkiDir, name))
		require.NoError(t, err)
		dates[name] = metav1.NewTime(cert.NotAfter)
	}
	for _, name := range []string{"admin.conf", "super-admin.conf"} {
		cert, err := kubeconfig.GetClientCertificateExpiration(filepath.Join(configDir, name))
		require.NoError(t, err)
		dates[name] = metav1.NewTime(cert.NotAfter)
	}
	return dates
}

func observationPath(pkiDir, configDir, name string) string {
	if filepath.Ext(name) == ".conf" {
		return filepath.Join(configDir, name)
	}
	return filepath.Join(pkiDir, name)
}
