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
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

const handoffTestNodeName = "example-master-0"

// handoffTestServer serves the given response over the material's certificate,
// the way the node does, and returns the address dhctl dials.
func handoffTestServer(t *testing.T, material *HandoffMaterial, handler http.HandlerFunc) string {
	t.Helper()

	server := immutabletest.HandoffServer(t, material.ServerCertPEM, material.ServerKeyPEM, handler)
	return strings.TrimPrefix(server.URL, "https://")
}

// TestFetchKubeconfig covers the shape of the channel: the endpoint is verified
// by the node's NAME while dhctl dials an address nobody knew when the
// certificate was issued.
func TestFetchKubeconfig(t *testing.T) {
	material, err := generateHandoffMaterial(handoffTestNodeName)
	require.NoError(t, err)

	var presented string
	address := handoffTestServer(t, material, func(w http.ResponseWriter, r *http.Request) {
		presented = r.Header.Get("Authorization")
		require.Equal(t, handoffPath, r.URL.Path)
		_, _ = w.Write([]byte("apiVersion: v1\nkind: Config\n"))
	})

	kubeconfig, err := FetchKubeconfig(t.Context(), FetchKubeconfigInput{
		Address:    address,
		ServerName: handoffTestNodeName,
		Material:   material,
	})
	require.NoError(t, err)
	require.Equal(t, "apiVersion: v1\nkind: Config\n", string(kubeconfig))
	require.Equal(t, "Bearer "+material.Token, presented)
}

// TestFetchKubeconfigVerifiesTheNodeName is the reason the certificate is issued
// for a name and not for an address: anything else answering on that address
// must not be able to hand dhctl a kubeconfig.
func TestFetchKubeconfigVerifiesTheNodeName(t *testing.T) {
	material, err := generateHandoffMaterial(handoffTestNodeName)
	require.NoError(t, err)

	address := handoffTestServer(t, material, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("apiVersion: v1\nkind: Config\n"))
	})

	_, err = FetchKubeconfig(t.Context(), FetchKubeconfigInput{
		Address:    address,
		ServerName: "somebody-else-master-0",
		Material:   material,
	})
	require.Error(t, err)

	other, err := generateHandoffMaterial(handoffTestNodeName)
	require.NoError(t, err)
	other.Token = material.Token

	_, err = FetchKubeconfig(t.Context(), FetchKubeconfigInput{
		Address:    address,
		ServerName: handoffTestNodeName,
		Material:   other,
	})
	require.Error(t, err, "a certificate from another CA must not be accepted")
}

// TestFetchKubeconfigHopelessAnswers separates "not up yet" from "will never
// work": both statuses here mean the caller must stop retrying.
func TestFetchKubeconfigHopelessAnswers(t *testing.T) {
	tests := []struct {
		name   string
		status int
		expErr error
	}{
		{name: "the tokens have diverged", status: http.StatusUnauthorized, expErr: ErrHandoffUnauthorized},
		{name: "somebody already collected them", status: http.StatusGone, expErr: ErrHandoffAlreadyServed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material, err := generateHandoffMaterial(handoffTestNodeName)
			require.NoError(t, err)

			address := handoffTestServer(t, material, func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "nope", tt.status)
			})

			_, err = FetchKubeconfig(t.Context(), FetchKubeconfigInput{
				Address:    address,
				ServerName: handoffTestNodeName,
				Material:   material,
			})
			require.ErrorIs(t, err, tt.expErr)
		})
	}
}

// TestHandoffMaterialIsReusedAcrossAttempts guards the retry: the node boots
// with the first payload, so a second attempt with fresh material would present
// a token the node rejects and verify against a CA the node never served under.
func TestHandoffMaterialIsReusedAcrossAttempts(t *testing.T) {
	stateCache := cache.NewTestCache()

	first, err := HandoffMaterialFor(t.Context(), stateCache, handoffTestNodeName)
	require.NoError(t, err)

	second, err := HandoffMaterialFor(t.Context(), stateCache, handoffTestNodeName)
	require.NoError(t, err)

	require.Equal(t, first, second)
}

// TestHandoffPayloadSurvivesTheConfirmedHandover guards the rerun after the
// confirmation drops the client key: the payload must still render byte for
// byte what the master booted with, or dhctl can no longer authenticate.
func TestHandoffPayloadSurvivesTheConfirmedHandover(t *testing.T) {
	stateCache := cache.NewTestCache()

	first, err := HandoffMaterialFor(t.Context(), stateCache, handoffTestNodeName)
	require.NoError(t, err)
	before, err := yaml.Marshal(handoffPayload(*first))
	require.NoError(t, err)

	require.NoError(t, ForgetHandoffClientKey(t.Context(), stateCache))

	second, err := HandoffMaterialFor(t.Context(), stateCache, handoffTestNodeName)
	require.NoError(t, err)
	after, err := yaml.Marshal(handoffPayload(*second))
	require.NoError(t, err)

	require.Equal(t, string(before), string(after))

	// And the one part that is not in the payload is gone: it is the private key
	// behind a cluster-admin certificate, and the cache outlives the bootstrap.
	require.NotEmpty(t, first.ClientKeyPEM)
	require.Empty(t, second.ClientKeyPEM)
}

// TestRetargetKubeconfig covers what the node cannot get right: it writes the
// kubeconfig for its own use, pointing at a loopback proxy that means nothing
// off the node.
func TestRetargetKubeconfig(t *testing.T) {
	const collected = `apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://127.0.0.1:6445
    certificate-authority-data: dGVzdA==
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
users:
- name: kubernetes-admin
  user:
    token: secret
`

	out, err := RetargetKubeconfig(t.Context(), []byte(collected), "https://192.168.1.10:6443", handoffTestNodeName)
	require.NoError(t, err)
	require.Contains(t, string(out), "server: https://192.168.1.10:6443")
	require.NotContains(t, string(out), "6445")
	require.Contains(t, string(out), "certificate-authority-data: dGVzdA==")

	// The apiserver's certificate is issued before the VM exists: it covers the
	// node's name, the node's own addresses and the operator's certSANs. The NAT
	// address a cloud hands out as the master's public one is in none of them, so
	// without the pinned name every call after the handoff fails the host check.
	kubeconfig, err := clientcmd.Load(out)
	require.NoError(t, err)
	require.Equal(t, handoffTestNodeName, kubeconfig.Clusters["kubernetes"].TLSServerName,
		"the dialled address is in no certificate; the node's name is in every one of them")

	// Required rather than optional: an empty name silently puts the check back on
	// the address, which is the failure this pins.
	_, err = RetargetKubeconfig(t.Context(), []byte(collected), "https://192.168.1.10:6443", "")
	require.Error(t, err)
}
