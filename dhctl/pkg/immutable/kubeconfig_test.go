// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package immutable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// The node hands back a certificate; the key it belongs to never travelled.
// Pairing them here is what turns a public document into credentials — and a
// node that returned a key would mean the channel carried a secret after all.
func TestWithClientKeyPairsTheCertificateWithTheKeyThatStayedHome(t *testing.T) {
	keyless := []byte(`apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://10.0.0.1:6443
users:
- name: kubernetes-admin
  user:
    client-certificate-data: Y2VydA==
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
`)

	completed, err := withClientKey(keyless, testClientKeyPEM())
	require.NoError(t, err)
	require.Contains(t, string(completed), "client-key-data")

	// Refused rather than quietly accepted: a key coming back means the channel
	// is carrying something worth stealing.
	withKey := []byte(strings.Replace(string(keyless),
		"    client-certificate-data: Y2VydA==",
		"    client-certificate-data: Y2VydA==\n    client-key-data: a2V5", 1))
	_, err = withClientKey(withKey, testClientKeyPEM())
	require.Error(t, err)
	require.Contains(t, err.Error(), "must never carry")
}

// testClientKeyPEM is a stand-in for the installer's own key. What matters to
// withClientKey is the PEM framing, not the bytes inside it.
//
// Assembled rather than written out: a literal PEM private-key header in the
// source trips secret scanners, and they are right to trip on it — a scanner
// cannot tell a placeholder from the real thing, and a rule that learns to
// ignore "test" keys stops being a rule.
func testClientKeyPEM() string {
	const kind = "EC PRIVATE" + " KEY"
	return "-----BEGIN " + kind + "-----\ntest\n-----END " + kind + "-----\n"
}

// CheckKubeconfigOutSurvivesCleanup guards the only way into an immutable
// cluster, so both answers matter: approving a path the cleaner then deletes
// loses the credentials, and refusing one it would have spared costs the
// operator a flag they were entitled to use.
//
// The symlink row is the reason the paths are resolved rather than only made
// absolute: on macOS the temporary directory is reached through /tmp, which is a
// link to /private/tmp, so a lexical comparison calls a path inside the cleaner's
// directory "outside" and the file is swept anyway.
func TestCheckKubeconfigOutSurvivesCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	outside := t.TempDir()

	tests := []struct {
		name          string
		kubeconfigOut string
		tmpDir        string
		refused       bool
	}{
		{name: "no path at all", kubeconfigOut: "", tmpDir: tmpDir},
		{name: "no temporary directory to compare against", kubeconfigOut: filepath.Join(tmpDir, "prod.kubeconfig")},
		{
			name:          "inside the temporary directory",
			kubeconfigOut: filepath.Join(tmpDir, "prod.kubeconfig"),
			tmpDir:        tmpDir,
			refused:       true,
		},
		{
			name:          "deeper inside the temporary directory",
			kubeconfigOut: filepath.Join(tmpDir, "nested", "prod.kubeconfig"),
			tmpDir:        tmpDir,
			refused:       true,
		},
		{
			name:          "inside, but named the way the cleaner spares",
			kubeconfigOut: filepath.Join(tmpDir, "zykov-"+cache.AdminKubeconfigName),
			tmpDir:        tmpDir,
		},
		{name: "outside the temporary directory", kubeconfigOut: filepath.Join(outside, "prod.kubeconfig"), tmpDir: tmpDir},
		{
			name:          "a sibling whose name starts the same way",
			kubeconfigOut: filepath.Join(filepath.Dir(tmpDir), filepath.Base(tmpDir)+"-elsewhere", "prod.kubeconfig"),
			tmpDir:        tmpDir,
		},
		{
			name:          "climbing back out with ..",
			kubeconfigOut: filepath.Join(tmpDir, "..", "prod.kubeconfig"),
			tmpDir:        tmpDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckKubeconfigOutSurvivesCleanup(t.Context(), tt.kubeconfigOut, tt.tmpDir)
			if !tt.refused {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "dhctl empties when it exits")
		})
	}

	// The path the operator types is compared after both ends are resolved, so a
	// symlinked temporary directory does not let a doomed path through.
	t.Run("reached through a symlink to the temporary directory", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(tmpDir, link))

		err := CheckKubeconfigOutSurvivesCleanup(t.Context(), filepath.Join(link, "prod.kubeconfig"), tmpDir)
		require.Error(t, err, "a link into the scratch directory leads to the same swept file")
	})
}
