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

package proxy

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func TestTrustBundleReadsWhatDeckhouseStagedOnTheNode(t *testing.T) {
	directory := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(directory, "b-registry-ca.crt"), []byte("SECOND\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "a-registry-ca.crt"), []byte("FIRST\n"), 0o644))
	// Not a certificate: the directory also holds whatever else was put there, and a
	// bundle is only what claims to be one.
	require.NoError(t, os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("IGNORED"), 0o644))

	bundle, err := loadTrustBundle(directory)
	require.NoError(t, err)
	assert.Equal(t, "FIRST\nSECOND\n", string(bundle),
		"read in a fixed order, so an unchanged directory produces an unchanged bundle")

	// A node where no module source was ever given a certificate has no such directory,
	// and that is the ordinary case rather than a failure.
	missing, err := loadTrustBundle(filepath.Join(directory, "absent"))
	require.NoError(t, err)
	assert.Empty(t, missing)
}

// TestPassThroughTrustsTheAuthoritiesStagedOnTheNode is the registry nobody configured,
// serving a certificate only this cluster's operator vouches for.
//
// A ModuleSource with its own certificate authority is the case that made this matter.
// Deckhouse stages that authority on every node — one file per registry, written by the
// bashible step that runs whatever implementation is in charge — and, before the agent,
// the runtime was told to use it directly, per registry. Under the agent every registry
// arrives at one `_default` drop-in instead, so those pulls are forwarded by the agent
// and verified against the AGENT's trust, which is its container image's bundle and
// contains nothing of the cluster's own.
//
// The result was a module source that could not be pulled from at all: the pull left the
// node, reached the agent, and failed at the handshake with an authority nobody had told
// the agent about — while the certificate sat on the node's filesystem all along.
func TestPassThroughTrustsTheAuthoritiesStagedOnTheNode(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Docker-Content-Digest", "sha256:module-source")
		_, _ = w.Write([]byte("a module image"))
	}))
	t.Cleanup(upstream.Close)

	// What step 003 leaves on the node, in the form it leaves it: the authority as PEM,
	// one file named after the registry.
	directory := t.TempDir()
	authority := pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw,
	})
	require.NoError(t, os.WriteFile(
		filepath.Join(directory, "modules-example-com-ca.crt"), authority, 0o644))

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendStorage,
			Endpoint: registryv1alpha1.Endpoint{Host: constant.Host, Path: constant.Path},
		}},
	}
	namespace := upstream.Listener.Addr().String()
	const path = "/v2/modules/some-module/manifests/v1.0.0"

	t.Run("without the staged authorities the pull cannot be verified", func(t *testing.T) {
		response := pull(t, newServer(spec), namespace, path)
		assert.NotEqual(t, http.StatusOK, response.StatusCode,
			"the agent has no reason to trust this registry, and must not invent one")
	})

	t.Run("with them the pull goes through", func(t *testing.T) {
		server := newServer(spec)
		server.TrustDir = directory
		server.RefreshTrust()

		response := pull(t, server, namespace, path)
		require.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "a module image", bodyOf(t, response))
	})

	// The directory is a mount of the node's, and a module source added while the agent
	// runs appears in it without anything restarting the agent.
	t.Run("an authority added later is picked up", func(t *testing.T) {
		later := t.TempDir()
		server := newServer(spec)
		server.TrustDir = later
		server.RefreshTrust()

		require.NoError(t, os.WriteFile(
			filepath.Join(later, "modules-example-com-ca.crt"), authority, 0o644))
		server.RefreshTrust()

		response := pull(t, server, namespace, path)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
}

// TestPassThroughToAReplicaIsVerifiedWithTheStoreAuthority: a repository outside the image set,
// named by a replica's address, is passed through without the node's credentials — and still has
// to reach the store. The replica's certificate is issued by the cluster's own authority, which
// is in the layout and nowhere on the node. Verified against the system trust alone, the request
// never got past the handshake, and the client saw a 502 whatever credentials it brought.
func TestPassThroughToAReplicaIsVerifiedWithTheStoreAuthority(t *testing.T) {
	const node, client = "registry-ro", "someone"

	var seen []string
	store := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, _ := r.BasicAuth()
		seen = append(seen, user)
		if user != client {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Docker-Content-Digest", "sha256:other")
		_, _ = w.Write([]byte("something pushed beside the image set"))
	}))
	t.Cleanup(store.Close)

	replica := store.Listener.Addr().String()
	authority := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: store.Certificate().Raw})
	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name: registryv1alpha1.BackendStorage,
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTPS,
				Host:   replica,
				Path:   constant.Path,
				CA:     string(authority),
				Auth:   &registryv1alpha1.Auth{Username: node, Password: "dh-test-secret-0123456789abcdef"},
			},
		}},
	}
	const path = "/v2/other/app/manifests/v1"

	t.Run("without credentials the store refuses it", func(t *testing.T) {
		seen = nil
		response := pull(t, newServer(spec), replica, path)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode,
			"the store's own answer, not a failed handshake")
		assert.NotContains(t, seen, node, "the node's account is for the image set alone")
	})

	t.Run("with the client's own credentials it goes through", func(t *testing.T) {
		seen = nil
		front := httptest.NewServer(newServer(spec).Handler())
		t.Cleanup(front.Close)

		request, err := http.NewRequest(http.MethodGet, front.URL+path+"?ns="+replica, nil)
		require.NoError(t, err)
		request.SetBasicAuth(client, "dh-test-secret-0123456789abcdef")

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() { _ = response.Body.Close() })

		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.NotContains(t, seen, node)
	})
}
