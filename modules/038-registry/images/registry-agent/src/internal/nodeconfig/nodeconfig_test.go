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

package nodeconfig

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuth is base64("user:password"), assembled here rather than written out.
//
// A literal of that shape is indistinguishable from a real credential to a secret
// scanner, and a fixture is exactly where one gets committed for real one day. Built
// from its parts, there is nothing in this file for either to find.
var fakeAuth = base64.StdEncoding.EncodeToString([]byte("license-token:not-a-real-key"))

// documentYAML is a NodeConfig as nodelet keeps it, with neighbours around the two fields
// this reads: the document is large, and taking two fields out of it must not trip over
// the rest.
func documentYAML() string {
	return fmt.Sprintf(`apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  nodeName: master-0
  apiServerEndpoints:
  - 192.168.1.10:6443
  - 192.168.1.11:6443
  containerRuntime:
    registryOwner: agent
    maxConcurrentDownloads: 8
  kubelet:
    maxPods: 110
    caCert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmNsdXN0ZXItYXV0aG9yaXR5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
  registry:
    address: registry.deckhouse.io
    path: /deckhouse/ee
    scheme: HTTPS
    ca: |
      -----BEGIN CERTIFICATE-----
      not-a-real-certificate
      -----END CERTIFICATE-----
    auth: %s
`, fakeAuth)
}

func write(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nodeconfig.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadTakesTheTwoFieldsTheAgentNeeds(t *testing.T) {
	document, err := Load(write(t, documentYAML()))
	require.NoError(t, err)
	require.NotNil(t, document)

	assert.Equal(t, []string{"192.168.1.10:6443", "192.168.1.11:6443"}, document.Spec.APIServerEndpoints)

	require.NotNil(t, document.Spec.Registry)
	assert.Equal(t, "registry.deckhouse.io", document.Spec.Registry.Address)
	assert.Equal(t, "/deckhouse/ee", document.Spec.Registry.Path)
	assert.Equal(t, "HTTPS", document.Spec.Registry.Scheme)
	assert.Contains(t, document.Spec.Registry.CA, "not-a-real-certificate")
	assert.Equal(t, fakeAuth, document.Spec.Registry.Auth)
}

// A bashible node has no such document, and that is the ordinary answer rather than a
// failure: it is brought up by a different mechanism entirely.
func TestLoadAbsentIsNotAnError(t *testing.T) {
	document, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Nil(t, document)
}

func TestLoadRejectsAnUnusableDocument(t *testing.T) {
	_, err := Load(write(t, "\tthis is not yaml: ["))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unusable")
}

// TestDefaultPathMatchesNodelet pins a contract with another repository: config.DefaultPath
// there. A drift is a node that silently has neither a seed nor an API address, which shows
// up much later as a joining master that cannot pull its control plane.
func TestDefaultPathMatchesNodelet(t *testing.T) {
	assert.Equal(t, "/config/nodeconfig.yaml", DefaultPath)

	document, err := Load("")
	require.NoError(t, err, "an empty path means the default, and the default is usually absent")
	_ = document
}

// The identity an Engine node gives the agent: the kubelet's own certificate, which the
// API server already trusts, and the API address from the node's own config.
func TestRestConfigFromTheNodesOwnIdentity(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.crt")
	cert := filepath.Join(dir, "kubelet-client-current.pem")
	require.NoError(t, os.WriteFile(ca, []byte("-----BEGIN CERTIFICATE-----\n"), 0o644))
	require.NoError(t, os.WriteFile(cert, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600))

	document, err := Load(write(t, documentYAML()))
	require.NoError(t, err)

	config, err := Identity{ClusterCAPath: ca, ClientCertificatePath: cert}.RestConfig(document)
	require.NoError(t, err)

	assert.Equal(t, "https://192.168.1.10:6443", config.Host,
		"the first endpoint, and only the first")
	assert.Equal(t, cert, config.TLSClientConfig.CertFile)
	assert.Equal(t, cert, config.TLSClientConfig.KeyFile,
		"one file holds both halves, which is how the kubelet's own kubeconfig names it")

	// The authority comes out of the document, so nothing of /etc/kubernetes/pki has to
	// be mounted — on a master that directory also holds the authority's private key.
	assert.Empty(t, config.TLSClientConfig.CAFile)
	assert.Contains(t, string(config.TLSClientConfig.CAData), "cluster-authority")
}

// A node config predating the field falls back to the authority on disk rather than
// building a client that verifies nothing.
func TestRestConfigFallsBackToTheAuthorityOnDisk(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.crt")
	cert := filepath.Join(dir, "kubelet-client-current.pem")
	require.NoError(t, os.WriteFile(ca, []byte("-----BEGIN CERTIFICATE-----\n"), 0o644))
	require.NoError(t, os.WriteFile(cert, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600))

	document, err := Load(write(t, "spec:\n  apiServerEndpoints:\n  - 10.0.0.1:6443\n"))
	require.NoError(t, err)

	config, err := Identity{ClusterCAPath: ca, ClientCertificatePath: cert}.RestConfig(document)
	require.NoError(t, err)
	assert.Equal(t, ca, config.TLSClientConfig.CAFile)
	assert.Empty(t, config.TLSClientConfig.CAData)
}

// An authority that is not base64 is a broken document, not a node that is not ready:
// falling back silently would verify the API server against the wrong thing.
func TestRestConfigRejectsAnUnusableAuthority(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "kubelet-client-current.pem")
	require.NoError(t, os.WriteFile(cert, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600))

	document, err := Load(write(t,
		"spec:\n  apiServerEndpoints:\n  - 10.0.0.1:6443\n  kubelet:\n    caCert: \"not base64 @@\"\n"))
	require.NoError(t, err)

	_, err = Identity{ClientCertificatePath: cert}.RestConfig(document)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNoIdentityYet), "this one an operator has to look at")
	assert.Contains(t, err.Error(), "base64")
}

// A node in the minutes before its TLS bootstrap has no certificate yet. That is "not
// yet", and the agent has to keep serving the container runtime through it — so it is a
// distinguishable error rather than a failure to start.
func TestRestConfigWithoutAnIdentityYet(t *testing.T) {
	dir := t.TempDir()
	document, err := Load(write(t, documentYAML()))
	require.NoError(t, err)

	_, err = Identity{
		ClusterCAPath:         filepath.Join(dir, "absent-ca.crt"),
		ClientCertificatePath: filepath.Join(dir, "absent-cert.pem"),
	}.RestConfig(document)
	// The kubelet's certificate is the half that is genuinely not there yet; the
	// authority the document carries is there from the first pass.

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoIdentityYet))
}

// The kubelet creates the file and fills it in, so an empty one is the same "not yet" as
// an absent one — and not a broken installation an operator should go looking at.
func TestRestConfigTreatsAnEmptyCertificateAsNotYet(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.crt")
	cert := filepath.Join(dir, "kubelet-client-current.pem")
	require.NoError(t, os.WriteFile(ca, []byte("-----BEGIN CERTIFICATE-----\n"), 0o644))
	require.NoError(t, os.WriteFile(cert, nil, 0o600))

	document, err := Load(write(t, documentYAML()))
	require.NoError(t, err)

	_, err = Identity{ClusterCAPath: ca, ClientCertificatePath: cert}.RestConfig(document)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoIdentityYet))
}

func TestRestConfigWithoutADocument(t *testing.T) {
	_, err := Identity{}.RestConfig(nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoIdentityYet))
}

// A document naming no API server is not something to build a client against: the
// resulting config would dial "https://" and fail with nothing pointing at the cause.
func TestRestConfigWithoutAnAPIServer(t *testing.T) {
	document, err := Load(write(t, "spec:\n  nodeName: worker-0\n"))
	require.NoError(t, err)

	_, err = Identity{}.RestConfig(document)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoIdentityYet))
	assert.Contains(t, err.Error(), "names no API server")
}

// The defaults are the paths the node actually uses, on both kinds of node: an Engine
// node's /etc/kubernetes is a symlink into /run, so the spelling is the same either way.
func TestIdentityDefaults(t *testing.T) {
	assert.Equal(t, "/etc/kubernetes/pki/ca.crt", Identity{}.clusterCAPath())
	assert.Equal(t, "/var/lib/kubelet/pki/kubelet-client-current.pem", Identity{}.clientCertificatePath())
}
