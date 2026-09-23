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

package checks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/settings"
	"github.com/deckhouse/lib-connection/pkg/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// proxiedRegistryHost is the address the registry is configured at. It is deliberately not a
// loopback one: NO_PROXY semantics exempt localhost and every loopback IP unconditionally, so a
// registry at 127.0.0.1 would take the direct path and never reach the proxy at all. The name
// matches the certificate httptest issues, so the TLS handshake on the far side of the proxy is
// verified for real.
const proxiedRegistryHost = "example.com"

// metaConfigWithProxy builds the configuration of a cluster behind a proxy, pointed at a registry
// this test controls.
func metaConfigWithProxy(t *testing.T, server *httptest.Server, proxyAddr string, noProxy []string) *config.MetaConfig {
	t.Helper()

	proxySection := map[string]any{"httpProxy": proxyAddr}
	if len(noProxy) > 0 {
		proxySection["noProxy"] = noProxy
	}

	clusterConfig := map[string]json.RawMessage{}
	for key, value := range map[string]any{
		"proxy":             proxySection,
		"clusterDomain":     "cluster.local",
		"podSubnetCIDR":     "10.111.0.0/16",
		"serviceSubnetCIDR": "10.222.0.0/16",
	} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		clusterConfig[key] = raw
	}

	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo(proxiedRegistryHost+"/deckhouse/ee"),
		registry_mocks.WithSchemeHTTPS(),
		registry_mocks.WithCA(serverCA(t, server)),
	)

	return &config.MetaConfig{Registry: registryCfg, ClusterConfig: clusterConfig}
}

// nodeInterfaceOf wraps the fake client the way lib-connection wraps a real one, so the check's
// own type assertion is the one that runs.
func nodeInterfaceOf(client *fakeSSHClient) NodeInterfaceFunc {
	wrapper := ssh.NewNodeInterfaceWrapper(client, settings.NewBaseProviders(settings.ProviderParams{}))
	return FixedNodeInterface(wrapper)
}

// TestRegistryProxyThroughAFakeTunnel runs the check end to end: a local forward standing in for
// the SSH one, a real HTTP proxy behind it, and a real TLS registry behind that. Every branch
// below used to be reachable only on a cluster.
func TestRegistryProxyThroughAFakeTunnel(t *testing.T) {
	tests := []struct {
		name string
		// handler answers /v2/ on the registry.
		handler http.HandlerFunc
		// refuseWith makes the proxy answer instead of connecting.
		refuseWith int
		wantDetail string
		wantErr    string
		wantFix    string
	}{
		{
			name: "the registry answers through the proxy",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				withRegistryAPIVersion(w)
				w.WriteHeader(http.StatusOK)
			},
			wantDetail: "answers from",
		},
		{
			name: "a registry that wants credentials still counts as reachable",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				withRegistryAPIVersion(w)
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantDetail: "answers from",
		},
		{
			// Something is at the address, but it is not a registry — a reverse proxy or a
			// captive portal serving its own page.
			name: "the answer is not from a registry API",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: "Docker-Distribution-API-Version",
		},
		{
			name: "the registry is up but broken",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			},
			wantErr: "HTTP 502",
			wantFix: "check that the proxy reaches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireTunnelPortFree(t)

			server, _ := registryServer(t, tt.handler)
			proxy := &fakeProxy{upstream: server.Listener.Addr().String(), refuseWith: tt.refuseWith}
			client := newFakeSSHClient(proxy.serve(t))

			check := RegistryProxyCheck{
				MetaConfig:    metaConfigWithProxy(t, server, "http://proxy.example.com:3128", nil),
				NodeInterface: nodeInterfaceOf(client),
			}

			detail, err := check.Run(context.Background())

			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Contains(t, detail, tt.wantDetail)
				assert.Equal(t, []string{"CONNECT " + proxiedRegistryHost + ":443"}, proxy.seen(),
					"the request must have gone through the proxy")
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			if tt.wantFix != "" {
				var failure *preflight.Failure
				require.ErrorAs(t, err, &failure)
				assert.Contains(t, failure.Fix, tt.wantFix)
			}
		})
	}
}

// TestRegistryProxyWhenTheProxyRefuses covers the two answers a proxy gives when it will not
// forward. Go reports a non-2xx answer to CONNECT as a transport error rather than a response, so
// these do not reach checkResponse at all — which is exactly why they need a real proxy to test.
func TestRegistryProxyWhenTheProxyRefuses(t *testing.T) {
	tests := []struct {
		name       string
		refuseWith int
		wantErr    string
		wantFix    string
	}{
		{
			name:       "the proxy wants credentials",
			refuseWith: http.StatusProxyAuthRequired,
			wantErr:    "HTTP 407",
			wantFix:    "ClusterConfiguration.proxy.httpsProxy",
		},
		{
			name:       "the proxy will not forward to this host",
			refuseWith: http.StatusForbidden,
			wantErr:    "HTTP 403",
			wantFix:    "allow " + proxiedRegistryHost,
		},
		{
			// Squid answers a CONNECT it cannot complete with 503, and that used to be
			// reported as though the registry itself had not answered.
			name:       "the proxy cannot reach the registry",
			refuseWith: http.StatusServiceUnavailable,
			wantErr:    "HTTP 503 from the proxy",
			wantFix:    "check that the proxy reaches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireTunnelPortFree(t)

			server, _ := registryServer(t, func(w http.ResponseWriter, _ *http.Request) {
				withRegistryAPIVersion(w)
				w.WriteHeader(http.StatusOK)
			})
			proxy := &fakeProxy{upstream: server.Listener.Addr().String(), refuseWith: tt.refuseWith}
			client := newFakeSSHClient(proxy.serve(t))

			check := RegistryProxyCheck{
				MetaConfig:    metaConfigWithProxy(t, server, "http://proxy.example.com:3128", nil),
				NodeInterface: nodeInterfaceOf(client),
			}

			_, err := check.Run(context.Background())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			// The reader has to be sent to the proxy, not to the registry: the registry
			// here is up and would have answered.
			var failure *preflight.Failure
			require.ErrorAs(t, err, &failure)
			assert.Contains(t, failure.Fix, tt.wantFix)
			assert.Contains(t, failure.Checked+failure.Observed, "proxy")
		})
	}
}

func TestRegistryProxyIsSkippedWhenItDoesNotApply(t *testing.T) {
	t.Run("no proxy is configured", func(t *testing.T) {
		check := RegistryProxyCheck{MetaConfig: &config.MetaConfig{}}

		_, err := check.Run(context.Background())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "no proxy is configured")
	})

	t.Run("noProxy exempts the registry", func(t *testing.T) {
		// The node would go direct, so checking the proxied path proves nothing. Deciding
		// this used to be exact string equality, which matched neither a leading dot nor a
		// CIDR — both documented, both common.
		server, _ := registryServer(t, func(http.ResponseWriter, *http.Request) {})
		check := RegistryProxyCheck{
			MetaConfig: metaConfigWithProxy(t, server, "http://proxy.example.com:3128", []string{proxiedRegistryHost}),
		}

		_, err := check.Run(context.Background())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "noProxy")
	})

	t.Run("a leading dot in noProxy covers subdomains only", func(t *testing.T) {
		// This is NO_PROXY as every tool implements it, the node included, so the check
		// has to agree: ".example.com" exempts registry.example.com and not example.com.
		// Pinned because it reads like a typo and is not one.
		server, _ := registryServer(t, func(http.ResponseWriter, *http.Request) {})
		check := RegistryProxyCheck{
			MetaConfig: metaConfigWithProxy(t, server, "http://proxy.example.com:3128", []string{"." + proxiedRegistryHost}),
		}

		_, err := check.Run(context.Background())

		assert.NotErrorIs(t, err, preflight.ErrNotApplicable,
			"the bare domain is not exempted by a dotted entry, so the proxy path is the one to check")
	})

	t.Run("there is no SSH connection to make the request from", func(t *testing.T) {
		server, _ := registryServer(t, func(http.ResponseWriter, *http.Request) {})
		check := RegistryProxyCheck{
			MetaConfig:    metaConfigWithProxy(t, server, "http://proxy.example.com:3128", nil),
			NodeInterface: FixedNodeInterface(nil),
		}

		_, err := check.Run(context.Background())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "no SSH host to make the request from")
	})
}

// TestRegistryProxyTunnelSpelling pins the two forwarding syntaxes. They are not interchangeable,
// and nothing else in the tree asserts which one a backend gets.
func TestRegistryProxyTunnelSpelling(t *testing.T) {
	tests := []struct {
		name       string
		legacyMode bool
		want       string
	}{
		{
			name:       "gossh forwards remote-to-local",
			legacyMode: false,
			want:       "proxy.example.com:3128:127.0.0.1:22323",
		},
		{
			name:       "clissh names the local port first",
			legacyMode: true,
			want:       "22323:proxy.example.com:3128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireTunnelPortFree(t)

			server, _ := registryServer(t, func(w http.ResponseWriter, _ *http.Request) {
				withRegistryAPIVersion(w)
				w.WriteHeader(http.StatusOK)
			})
			proxy := &fakeProxy{upstream: server.Listener.Addr().String()}
			client := newFakeSSHClient(proxy.serve(t))

			check := RegistryProxyCheck{
				MetaConfig:    metaConfigWithProxy(t, server, "http://proxy.example.com:3128", nil),
				NodeInterface: nodeInterfaceOf(client),
				LegacyMode:    tt.legacyMode,
			}

			_, err := check.Run(context.Background())
			require.NoError(t, err)

			assert.Equal(t, []string{tt.want}, client.tunnelSpecs())
		})
	}
}

// TestRegistryProxyWhenTheForwardIsRefused is the AllowTcpForwarding case: sshd will not open the
// port, and the advice has to point at sshd rather than at the registry.
func TestRegistryProxyWhenTheForwardIsRefused(t *testing.T) {
	server, _ := registryServer(t, func(http.ResponseWriter, *http.Request) {})
	client := newFakeSSHClient("127.0.0.1:1")
	client.tunnelErr = errors.New("ssh: tcpip-forward request denied by peer")

	check := RegistryProxyCheck{
		MetaConfig:    metaConfigWithProxy(t, server, "http://proxy.example.com:3128", nil),
		NodeInterface: nodeInterfaceOf(client),
	}

	_, err := check.Run(context.Background())

	require.Error(t, err)
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Checked, "ssh port forward")
	assert.Contains(t, failure.Fix, "AllowTcpForwarding")
	assert.Contains(t, failure.Fix, "22323")
}

// TestRegistryProxyWithABadCA: a CA that does not parse is a mistake in the configuration, so the
// check must say so once and not spend its retries on it.
func TestRegistryProxyWithABadCA(t *testing.T) {
	requireTunnelPortFree(t)

	server, _ := registryServer(t, func(http.ResponseWriter, *http.Request) {})
	proxy := &fakeProxy{upstream: server.Listener.Addr().String()}
	client := newFakeSSHClient(proxy.serve(t))

	meta := metaConfigWithProxy(t, server, "http://proxy.example.com:3128", nil)
	meta.Registry = registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo(proxiedRegistryHost+"/deckhouse/ee"),
		registry_mocks.WithSchemeHTTPS(),
		registry_mocks.WithCA("-----BEGIN CERTIFICATE-----\nnot a certificate\n-----END CERTIFICATE-----"),
	)

	check := RegistryProxyCheck{MetaConfig: meta, NodeInterface: nodeInterfaceOf(client)}

	_, err := check.Run(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid PEM bundle")

	// A malformed CA cannot be retried into working, and the runner is told so by the error
	// being wrapped as permanent.
	var permanent *backoff.PermanentError
	assert.ErrorAs(t, err, &permanent)
}
