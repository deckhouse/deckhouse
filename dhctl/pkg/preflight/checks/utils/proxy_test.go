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

package utils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// TestShouldSkipProxyCheck covers the NO_PROXY syntax the node itself applies. The previous
// implementation compared strings for equality and resolved both sides to an IP, so the two forms
// operators write most — a domain suffix and a CIDR block — matched nothing: the check went
// through the proxy on an endpoint the node would have reached directly, and reported a proxy
// problem that did not exist.
func TestShouldSkipProxyCheck(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		noProxy  []string
		want     bool
	}{
		{"no noProxy at all", "https://registry.example.com/v2/", nil, false},
		{"an exact host", "https://registry.example.com/v2/", []string{"registry.example.com"}, true},
		{"an unrelated host", "https://registry.example.com/v2/", []string{"other.example.com"}, false},
		{"a bare domain covers its subdomains", "https://registry.example.com/v2/", []string{"example.com"}, true},
		{"a leading dot covers subdomains", "https://registry.example.com/v2/", []string{".example.com"}, true},
		{"a leading dot does not cover the domain itself", "https://example.com/v2/", []string{".example.com"}, false},
		{"a CIDR block covers an address inside it", "https://10.20.30.40:5000/v2/", []string{"10.0.0.0/8"}, true},
		{"a CIDR block does not cover an address outside it", "https://192.168.1.1:5000/v2/", []string{"10.0.0.0/8"}, false},
		{"a host:port entry matches that port", "https://registry.example.com:5000/v2/", []string{"registry.example.com:5000"}, true},
		{"a host:port entry does not match another port", "https://registry.example.com:6000/v2/", []string{"registry.example.com:5000"}, false},
		{"a star exempts everything", "https://registry.example.com/v2/", []string{"*"}, true},
		{"one entry of several matches", "https://registry.example.com/v2/", []string{"10.0.0.0/8", ".example.com"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := url.Parse(tt.endpoint)
			if err != nil {
				t.Fatalf("bad test URL: %v", err)
			}
			if got := ShouldSkipProxyCheck(endpoint, tt.noProxy); got != tt.want {
				t.Errorf("ShouldSkipProxyCheck(%q, %v) = %v, want %v", tt.endpoint, tt.noProxy, got, tt.want)
			}
		})
	}
}

// TestBuildHTTPClientWithLocalhostProxyDoesNotMutateTheProxyURL: the URL belongs to the caller and
// is read again by whatever else consults the proxy configuration. Pointing its Host at localhost
// in place left every later reader with a proxy address of "localhost:22323".
func TestBuildHTTPClientWithLocalhostProxyDoesNotMutateTheProxyURL(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.corp:3128")
	if err != nil {
		t.Fatalf("bad test URL: %v", err)
	}

	if _, err := BuildHTTPClientWithLocalhostProxy(proxyURL, TLSOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := proxyURL.String(); got != "http://proxy.corp:3128" {
		t.Errorf("the caller's URL was rewritten to %q", got)
	}
}

// TestBuildTLSConfigRefusesAnUnreadableCA: falling back to the system pool would let the run
// proceed to a verification failure the operator believes they have already fixed.
func TestBuildTLSConfigRefusesAnUnreadableCA(t *testing.T) {
	if _, err := BuildTLSConfig(TLSOptions{CACert: "this is not a certificate"}); err == nil {
		t.Error("a CA that is not PEM must be an error")
	}

	cfg, err := BuildTLSConfig(TLSOptions{ServerName: "registry.example.com", Insecure: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerName != "registry.example.com" {
		t.Errorf("ServerName = %q; the tunnel makes every request look like localhost, so it has to be set", cfg.ServerName)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("the provider configuration asked for insecure and it was dropped")
	}
}

// TestGetProxyFromMetaConfig reads the proxy out of ClusterConfiguration. Everything downstream —
// which address the tunnel goes to, whether the registry check applies at all — is decided by what
// this returns, and it had no test.
func TestGetProxyFromMetaConfig(t *testing.T) {
	metaConfig := func(t *testing.T, cluster map[string]any) *config.MetaConfig {
		t.Helper()

		raw := map[string]json.RawMessage{}
		for key, value := range cluster {
			encoded, err := json.Marshal(value)
			require.NoError(t, err)
			raw[key] = encoded
		}
		return &config.MetaConfig{ClusterConfig: raw}
	}

	// EnrichProxyData needs these three whenever a proxy section exists: it adds the cluster's
	// own networks to noProxy so in-cluster traffic never goes through the proxy.
	withNetworks := func(proxy any) map[string]any {
		return map[string]any{
			"proxy":             proxy,
			"clusterDomain":     "cluster.local",
			"podSubnetCIDR":     "10.111.0.0/16",
			"serviceSubnetCIDR": "10.222.0.0/16",
		}
	}

	t.Run("no proxy section at all", func(t *testing.T) {
		proxyURL, noProxy, err := GetProxyFromMetaConfig(metaConfig(t, map[string]any{}))

		require.NoError(t, err)
		assert.Nil(t, proxyURL, "no proxy means the checks that need one do not apply")
		assert.Empty(t, noProxy)
	})

	t.Run("httpsProxy wins over httpProxy", func(t *testing.T) {
		// Both may be set, and the registry is reached over https.
		proxyURL, _, err := GetProxyFromMetaConfig(metaConfig(t, withNetworks(map[string]any{
			"httpProxy":  "http://plain.example.com:3128",
			"httpsProxy": "https://secure.example.com:3129",
		})))

		require.NoError(t, err)
		require.NotNil(t, proxyURL)
		assert.Equal(t, "secure.example.com:3129", proxyURL.Host)
	})

	t.Run("httpProxy on its own", func(t *testing.T) {
		proxyURL, _, err := GetProxyFromMetaConfig(metaConfig(t, withNetworks(map[string]any{
			"httpProxy": "http://user:password@plain.example.com:3128",
		})))

		require.NoError(t, err)
		require.NotNil(t, proxyURL)
		assert.Equal(t, "plain.example.com:3128", proxyURL.Host)
		// The credentials come through — and must never reach a log unredacted.
		assert.Equal(t, "http://user:xxxxx@plain.example.com:3128", proxyURL.Redacted())
	})

	t.Run("the cluster's own networks are always exempt", func(t *testing.T) {
		_, noProxy, err := GetProxyFromMetaConfig(metaConfig(t, withNetworks(map[string]any{
			"httpProxy": "http://plain.example.com:3128",
			"noProxy":   []string{"registry.company.my"},
		})))

		require.NoError(t, err)
		assert.Contains(t, noProxy, "registry.company.my")
		for _, always := range []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"} {
			assert.Contains(t, noProxy, always, "in-cluster traffic must never go through the proxy")
		}
	})

	t.Run("a proxy section with no address", func(t *testing.T) {
		_, _, err := GetProxyFromMetaConfig(metaConfig(t, withNetworks(map[string]any{
			"noProxy": []string{"example.com"},
		})))

		require.ErrorIs(t, err, ErrBadProxyConfig)
		assert.Contains(t, err.Error(), "no proxy address")
	})

	t.Run("an address that is not a URL", func(t *testing.T) {
		_, _, err := GetProxyFromMetaConfig(metaConfig(t, withNetworks(map[string]any{
			"httpProxy": "http://%zz",
		})))

		require.ErrorIs(t, err, ErrBadProxyConfig)
	})
}

// TestBuildHTTPClientThroughTunnel: the direct path dials the local end of the tunnel whatever
// host the URL names, which is the only way a request for an address only the node can resolve
// can be made from here.
func TestBuildHTTPClientThroughTunnel(t *testing.T) {
	// The port is fixed in production, so the stand-in for the tunnel's near end has to bind
	// the same one. If something else on the machine holds it, skipping beats a failure that
	// says nothing about the code.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", ProxyTunnelPort))
	if err != nil {
		t.Skipf("port %s is in use on this host", ProxyTunnelPort)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What the far end sees is the name from the URL, not the loopback address dialed.
		_, _ = io.WriteString(w, r.Host)
	}))
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	client, err := BuildHTTPClientThroughTunnel(TLSOptions{})
	require.NoError(t, err)

	// An address that resolves nowhere: the dialer is what makes this reachable at all.
	resp, err := client.Get("http://cloud-api.internal.invalid/version")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "cloud-api.internal.invalid", string(body),
		"the Host header keeps the real name so the far end routes correctly")
}

// TestSetupSSHTunnelToProxyAddr pins the two forwarding syntaxes and the default ports. The two
// SSH backends spell a port-forward differently and nothing else asserts which one a backend gets.
func TestSetupSSHTunnelToProxyAddr(t *testing.T) {
	tests := []struct {
		name       string
		proxy      string
		legacyMode bool
		want       string
	}{
		{
			name:  "gossh forwards remote-to-local",
			proxy: "http://proxy.example.com:3128",
			want:  "proxy.example.com:3128:127.0.0.1:" + ProxyTunnelPort,
		},
		{
			name:       "clissh names the local port first",
			proxy:      "http://proxy.example.com:3128",
			legacyMode: true,
			want:       ProxyTunnelPort + ":proxy.example.com:3128",
		},
		{
			// A proxy written without a port: the scheme decides, and getting it wrong
			// forwards to port 0.
			name:  "http defaults to 80",
			proxy: "http://proxy.example.com",
			want:  "proxy.example.com:80:127.0.0.1:" + ProxyTunnelPort,
		},
		{
			name:  "https defaults to 443",
			proxy: "https://proxy.example.com",
			want:  "proxy.example.com:443:127.0.0.1:" + ProxyTunnelPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyURL, err := url.Parse(tt.proxy)
			require.NoError(t, err)

			client := &recordingSSHClient{}
			tunnel, err := SetupSSHTunnelToProxyAddr(t.Context(), client, proxyURL, tt.legacyMode)

			require.NoError(t, err)
			require.NotNil(t, tunnel)
			assert.Equal(t, tt.want, client.asked)
		})
	}

	t.Run("the forward is refused", func(t *testing.T) {
		proxyURL, err := url.Parse("http://proxy.example.com:3128")
		require.NoError(t, err)

		client := &recordingSSHClient{upErr: errors.New("ssh: tcpip-forward request denied by peer")}
		tunnel, err := SetupSSHTunnelToProxyAddr(t.Context(), client, proxyURL, false)

		require.Error(t, err)
		assert.Nil(t, tunnel, "a tunnel that never came up must not be handed back to be closed")
	})
}

// recordingSSHClient is an SSH client that only remembers what forward it was asked for. The
// embedded nil interface makes any other method panic rather than answer.
type recordingSSHClient struct {
	libcon.SSHClient

	asked string
	upErr error
}

func (c *recordingSSHClient) Tunnel(address string) libcon.Tunnel {
	c.asked = address
	return &recordingTunnel{upErr: c.upErr}
}

type recordingTunnel struct {
	libcon.Tunnel

	upErr error
}

func (t *recordingTunnel) Up(context.Context) error { return t.upErr }
func (t *recordingTunnel) Stop()                    {}
