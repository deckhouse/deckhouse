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

package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKubeConfig = `apiVersion: v1
kind: Config
current-context: ctx-a
clusters:
- name: cluster-a
  cluster:
    server: https://a.example:6443
    insecure-skip-tls-verify: true
- name: cluster-b
  cluster:
    server: https://b.example:6443
    insecure-skip-tls-verify: true
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
users:
- name: user-a
  user:
    token: token-a
- name: user-b
  user:
    token: token-b
`

func writeTestKubeConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(path, []byte(testKubeConfig), 0o600))

	return path
}

func TestRESTConfigFromKubeConfig(t *testing.T) {
	path := writeTestKubeConfig(t)

	tests := []struct {
		give     string // context name, empty means current-context
		wantHost string
	}{
		{give: "", wantHost: "https://a.example:6443"},
		{give: "ctx-b", wantHost: "https://b.example:6443"},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			config, err := RESTConfig(WithKubeConfig(path), WithKubeContext(tt.give))
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, config.Host)
		})
	}
}

func TestSetDefaultKubeConfig(t *testing.T) {
	path := writeTestKubeConfig(t)

	// SetDefaultKubeConfig mutates package-level defaults, so restore them.
	t.Cleanup(func() { SetDefaultKubeConfig("", "") })

	SetDefaultKubeConfig(path, "ctx-b")

	config, err := RESTConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://b.example:6443", config.Host)
}
