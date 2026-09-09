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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "deckhouse.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadReadsWhatTheSyncerRenders(t *testing.T) {
	path := write(t, `
scope: system/deckhouse
upstream:
  address: registry.deckhouse.io
  scheme: HTTPS
  path: /deckhouse/ee
  ca: /pki/upstream-registry-ca.crt
  username: license-token
  password: secret
writeEndpoint:
  address: 0.0.0.0:5003
  clientCertCA: /pki/ingress-client-ca.crt
authProxy:
  url: https://127.0.0.1:5051/auth
  ca: /pki/ca.crt
`)

	wrapper, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "system/deckhouse", wrapper.Scope)
	require.NotNil(t, wrapper.Upstream)
	assert.Equal(t, "registry.deckhouse.io", wrapper.Upstream.Address)
	assert.Equal(t, "/deckhouse/ee", wrapper.Upstream.Path)
	assert.Equal(t, "0.0.0.0:5003", wrapper.WriteEndpoint.Address)
	require.NotNil(t, wrapper.AuthProxy)
	assert.Equal(t, "https://127.0.0.1:5051/auth", wrapper.AuthProxy.URL)

	// The two prefixes the mapping is made of, which is the whole reason this file exists.
	assert.Equal(t, "/v2/system/deckhouse", wrapper.LocalPrefix())
	assert.Equal(t, "/v2/deckhouse/ee", wrapper.RemotePrefix())
}

// TestAnAirGappedClusterHasNoUpstreamAtAll: absent, not empty. The store is then authoritative, and
// the difference is what the whole air-gap transition turns on.
func TestAnAirGappedClusterHasNoUpstreamAtAll(t *testing.T) {
	wrapper, err := Load(write(t, "scope: system/deckhouse\nwriteEndpoint:\n  address: 0.0.0.0:5003\n"))
	require.NoError(t, err)

	assert.Nil(t, wrapper.Upstream)
	assert.Empty(t, wrapper.RemotePrefix(), "there is nothing to map towards")
}

// TestAnUpstreamServingTheSameLayoutNeedsNoPath is the ordinary mirror: it holds the image set under
// the same prefix the cluster uses, so the mapping is the identity.
func TestAnUpstreamServingTheSameLayoutNeedsNoPath(t *testing.T) {
	wrapper, err := Load(write(t, `
scope: system/deckhouse
upstream:
  address: mirror.example.com
`))
	require.NoError(t, err)
	assert.Equal(t, "/v2", wrapper.RemotePrefix())
}

func TestLoadRefusesWhatCannotWork(t *testing.T) {
	cases := map[string]struct {
		content  string
		contains string
	}{
		"no scope": {
			// Without it nothing knows which requests are the cluster's, and every pull would be
			// forwarded to the upstream unmapped.
			content:  "writeEndpoint:\n  address: 0.0.0.0:5003\n",
			contains: "scope is required",
		},
		"an upstream with no address": {
			content:  "scope: system/deckhouse\nupstream:\n  path: /deckhouse/ee\n",
			contains: "upstream.address is required",
		},
		"a scheme that is neither": {
			content:  "scope: system/deckhouse\nupstream:\n  address: registry\n  scheme: ftp\n",
			contains: "neither http nor https",
		},
		"a token service with no address": {
			content:  "scope: system/deckhouse\nauthProxy:\n  ca: /pki/ca.crt\n",
			contains: "authProxy.url is required",
		},
		"a client authority with nothing to present it": {
			// The authority only means anything on a listener that asks for a certificate.
			content:  "scope: system/deckhouse\nwriteEndpoint:\n  clientCertCA: /pki/ca.crt\n",
			contains: "without writeEndpoint.address",
		},
		"a key this build does not know": {
			// The syncer renders this file, so an unknown key means the two halves of one image
			// disagree — and carrying on with it ignored is how a cluster serves something nobody
			// asked for.
			content:  "scope: system/deckhouse\nupstrem:\n  address: typo\n",
			contains: "field upstrem not found",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, testCase.content))
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.contains)
		})
	}
}

func TestLoadSaysWhichFileIsMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absent.yaml")
}
