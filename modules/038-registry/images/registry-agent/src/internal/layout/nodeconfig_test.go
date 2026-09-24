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

package layout

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// nodeConfigDocumentYAML is a NodeConfig as nodelet keeps it on the node, trimmed to the
// fields around the one this reads. The neighbours are kept deliberately: the document is
// large and this must take its one field out of it without tripping over the rest.
const nodeConfigDocumentYAML = `apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  nodeName: master-0
  apiServerEndpoints:
  - 192.168.1.10:6443
  containerRuntime:
    registryOwner: agent
    maxConcurrentDownloads: 8
  registry:
    address: registry.deckhouse.io
    path: /deckhouse/ee
    scheme: HTTPS
    ca: |
      -----BEGIN CERTIFICATE-----
      not-a-real-certificate
      -----END CERTIFICATE-----
    auth: %s
`

func writeNodeConfig(t *testing.T, body string) nodeConfigSeed {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nodeconfig.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return nodeConfigSeed{Path: path}
}

// The seed an Engine node starts from: the registry its own config names, as one upstream
// backend with the credentials carried across unchanged.
func TestNodeConfigSeedTakesTheNodesOwnRegistry(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("license-token:the-license-key"))
	seed := writeNodeConfig(t, fmt.Sprintf(nodeConfigDocumentYAML, auth))

	spec, err := seed.load()
	require.NoError(t, err)
	require.NotNil(t, spec)

	assert.False(t, spec.Cache,
		"the seed must not claim the in-cluster store, which may not exist yet")

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, registryv1alpha1.SchemeHTTPS, upstream.Scheme)
	assert.Equal(t, "registry.deckhouse.io", upstream.Host)
	assert.Equal(t, "/deckhouse/ee", upstream.Path)
	assert.Contains(t, upstream.CA, "not-a-real-certificate")

	require.NotNil(t, upstream.Auth)
	assert.Equal(t, auth, upstream.Auth.Auth,
		"base64(user:password) is the form both sides keep it in, so it is copied, not parsed")
}

// An ordinary bashible node has no such document, and that is not a failure: it has a
// seed of its own, or an API server to ask.
func TestNodeConfigSeedAbsentIsNotAnError(t *testing.T) {
	seed := nodeConfigSeed{Path: filepath.Join(t.TempDir(), "absent.yaml")}

	spec, err := seed.load()
	require.NoError(t, err)
	assert.Nil(t, spec)
}

// A node told to reach no registry directly seeds nothing rather than seeding a backend
// with an empty host, which the runtime would be pointed at and could never reach.
func TestNodeConfigSeedWithoutARegistry(t *testing.T) {
	seed := writeNodeConfig(t, "apiVersion: internal.deckhouse.io/v1alpha1\nkind: NodeConfig\nspec:\n  nodeName: worker-0\n")

	spec, err := seed.load()
	require.NoError(t, err)
	assert.Nil(t, spec)
}

// The scheme has a CRD default, and a document read from a file passes no API server to
// apply it — the same reason nodelet's own loader applies it in code.
func TestNodeConfigSeedDefaultsTheScheme(t *testing.T) {
	seed := writeNodeConfig(t,
		"spec:\n  registry:\n    address: registry.example.com:5000\n")

	spec, err := seed.load()
	require.NoError(t, err)
	require.NotNil(t, spec)

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, registryv1alpha1.SchemeHTTPS, upstream.Scheme)
	assert.Nil(t, upstream.Auth, "no credentials is anonymous, not empty credentials")
}

func TestNodeConfigSeedRejectsAnUnusableDocument(t *testing.T) {
	seed := writeNodeConfig(t, "\tthis is not yaml: [")

	_, err := seed.load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unusable")
}

// The file wins over the node config when a node has both. It was written for this agent
// by whoever installed it; the node config only describes how the node reaches a registry.
func TestBootstrapPrefersItsFileOverTheNodeConfig(t *testing.T) {
	seed := writeSeed(t, seedSpec())
	nodeConfig := writeNodeConfig(t, fmt.Sprintf(nodeConfigDocumentYAML, "Zm9vOmJhcg=="))
	seed.NodeConfigPath = nodeConfig.Path

	spec, err := seed.Load()
	require.NoError(t, err)
	require.NotNil(t, spec)

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, "the-license-key", upstream.Auth.Password,
		"the file's credentials, not the node config's")
}

// And it falls through to the node config when there is no file, which is every Engine
// node: nothing writes one there.
func TestBootstrapFallsThroughToTheNodeConfig(t *testing.T) {
	dir := t.TempDir()
	nodeConfig := writeNodeConfig(t, fmt.Sprintf(nodeConfigDocumentYAML, "Zm9vOmJhcg=="))

	seed := &Bootstrap{
		Path:           filepath.Join(dir, "absent.json"),
		NodeConfigPath: nodeConfig.Path,
	}

	spec, err := seed.Load()
	require.NoError(t, err)
	require.NotNil(t, spec)

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, "registry.deckhouse.io", upstream.Host)
}
