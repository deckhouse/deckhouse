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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// cloudAPIHost is the address the provider API is configured at — not a loopback one, for the
// reason proxiedRegistryHost is not, and matching the certificate httptest issues so the TLS
// handshake through the tunnel is verified rather than skipped.
const cloudAPIHost = "example.com"

// cloudAPIServer starts a TLS server standing in for the provider API and returns a MetaConfig
// pointed at it. vcd is used because its endpoint is a plain field of the provider section, so
// the address and the CA can both be set from here.
func cloudAPIServer(t *testing.T, handler http.HandlerFunc, proxyAddr string, noProxy []string) (*httptest.Server, *config.MetaConfig) {
	t.Helper()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	providerConfig := map[string]any{
		"server":   "https://" + cloudAPIHost,
		"caBundle": base64.StdEncoding.EncodeToString([]byte(serverCA(t, server))),
	}
	providerJSON, err := json.Marshal(providerConfig)
	require.NoError(t, err)

	meta := metaConfigForProvider(t, "vcd", string(providerJSON))

	if proxyAddr != "" {
		if meta.ClusterConfig == nil {
			meta.ClusterConfig = map[string]json.RawMessage{}
		}
		proxySection := map[string]any{"httpProxy": proxyAddr}
		if len(noProxy) > 0 {
			proxySection["noProxy"] = noProxy
		}
		for key, value := range map[string]any{
			"proxy":             proxySection,
			"clusterDomain":     "cluster.local",
			"podSubnetCIDR":     "10.111.0.0/16",
			"serviceSubnetCIDR": "10.222.0.0/16",
		} {
			raw, err := json.Marshal(value)
			require.NoError(t, err)
			meta.ClusterConfig[key] = raw
		}
	}

	return server, meta
}

// TestCloudAPIThroughATunnel runs the check with the SSH forward stood up locally. Before this the
// check had no test that reached its request at all: everything below the endpoint lookup was
// exercised only against a cloud.
func TestCloudAPIThroughATunnel(t *testing.T) {
	// Any answer means the node has egress, which is the whole question. 401 and 404 are
	// passes; this used to be the check that "silently passed" on a connection error instead.
	answers := []struct {
		name   string
		status int
		wantOK bool
	}{
		{name: "the API answers", status: http.StatusOK, wantOK: true},
		{name: "the API wants credentials", status: http.StatusUnauthorized, wantOK: true},
		{name: "the address is not an API path", status: http.StatusNotFound, wantOK: true},
		{name: "the API is broken", status: http.StatusBadGateway, wantOK: false},
	}

	for _, answer := range answers {
		t.Run(answer.name, func(t *testing.T) {
			requireTunnelPortFree(t)

			server, meta := cloudAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(answer.status)
			}, "", nil)

			// No proxy is configured, so the tunnel goes straight to the API and the
			// client dials its local end whatever host the URL names.
			client := newFakeSSHClient(server.Listener.Addr().String())
			check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

			detail, err := check.Run(context.Background())

			if answer.wantOK {
				require.NoError(t, err)
				assert.Contains(t, detail, "answers from the master node")
				assert.NotContains(t, detail, "via proxy")
				return
			}

			require.Error(t, err)
			var failure *preflight.Failure
			require.ErrorAs(t, err, &failure)
			assert.Contains(t, failure.Observed, fmt.Sprintf("HTTP %d", answer.status))
			assert.Contains(t, failure.Fix, "egress")
		})
	}
}

// TestCloudAPIThroughAProxy: with a proxy configured the tunnel goes to the proxy, not to the API,
// and the report has to say so — the two are different things to go and look at.
func TestCloudAPIThroughAProxy(t *testing.T) {
	requireTunnelPortFree(t)

	server, meta := cloudAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, "http://proxy.example.com:3128", nil)

	proxy := &fakeProxy{upstream: server.Listener.Addr().String()}
	client := newFakeSSHClient(proxy.serve(t))
	check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

	detail, err := check.Run(context.Background())

	require.NoError(t, err)
	assert.Contains(t, detail, "via proxy http://proxy.example.com:3128")
	assert.Equal(t, []string{"CONNECT " + cloudAPIHost + ":443"}, proxy.seen())
	// The forward has to reach the proxy, since that is what the request is sent to.
	assert.Equal(t, []string{"proxy.example.com:3128:127.0.0.1:22323"}, client.tunnelSpecs())
}

// TestCloudAPIWhenNoProxyExemptsIt: the endpoint is exempt, so the node goes direct — and so must
// the tunnel. Sending it to the proxy would test a path the node never takes.
func TestCloudAPIWhenNoProxyExemptsIt(t *testing.T) {
	requireTunnelPortFree(t)

	server, meta := cloudAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, "http://proxy.example.com:3128", []string{cloudAPIHost})

	proxy := &fakeProxy{upstream: server.Listener.Addr().String()}
	proxy.serve(t)
	client := newFakeSSHClient(server.Listener.Addr().String())
	check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

	detail, err := check.Run(context.Background())

	require.NoError(t, err)
	assert.NotContains(t, detail, "via proxy")
	assert.Empty(t, proxy.seen(), "the proxy must not have been used")
	assert.Equal(t, []string{cloudAPIHost + ":443:127.0.0.1:22323"}, client.tunnelSpecs(),
		"the forward goes to the API itself when noProxy exempts it")
}

// TestCloudAPIWhenTheProxyRefuses is the case a cloud API can only hit through an https CONNECT,
// where the refusal arrives as a transport error and used to be reported as the API being
// unreachable.
func TestCloudAPIWhenTheProxyRefuses(t *testing.T) {
	requireTunnelPortFree(t)

	server, meta := cloudAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, "http://proxy.example.com:3128", nil)

	proxy := &fakeProxy{upstream: server.Listener.Addr().String(), refuseWith: http.StatusProxyAuthRequired}
	client := newFakeSSHClient(proxy.serve(t))
	check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

	_, err := check.Run(context.Background())

	require.Error(t, err)
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "HTTP 407")
	assert.Contains(t, failure.Fix, "ClusterConfiguration.proxy.httpsProxy")

	// Credentials the proxy rejects will be rejected again; the runner must not spend its
	// retries on it.
	var permanent *backoff.PermanentError
	assert.ErrorAs(t, err, &permanent)
}

// TestCloudAPIDoesNotProbeTheConnectionItself: the check used to wait for the master to answer
// before doing anything, because nothing else in this phase did. ssh-credential goes first now and
// carries that wait, so probing again re-proved a connection that had just been proven — and put a
// second "Waiting for SSH connection" box in the middle of the report, which read as though the
// wait were happening twice.
func TestCloudAPIDoesNotProbeTheConnectionItself(t *testing.T) {
	requireTunnelPortFree(t)

	server, meta := cloudAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, "", nil)

	client := newFakeSSHClient(server.Listener.Addr().String())
	// An availability probe would fail on this; the check must never ask for one.
	client.onAwait = func(retry.Params) { t.Error("the connection is proven by ssh-credential, not here") }
	client.reachErr = errors.New("the probe must not be reached")

	check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

	_, err := check.Run(context.Background())

	require.NoError(t, err)
}

// exitError returns a real *exec.ExitError with the given status, which is what the clissh backend
// hands back: it runs the ssh binary and passes on what Wait() returned.
func exitError(t *testing.T, status int) error {
	t.Helper()

	err := exec.Command("sh", "-c", fmt.Sprintf("exit %d", status)).Run()
	require.Error(t, err)
	return err
}

// TestCloudAPIWithTheWrongSSHUser is the failure this check was reported for: on an OpenStack
// cluster the master is created, the login user of the image is not the one that was passed, and
// every message the operator got was about something else — the last of them advising them to
// check AllowTcpForwarding on a machine they had never logged in to.
//
// ssh-credential answers this first now and stops the phase, so this is the path left when it was
// turned off by name. The check cannot know it was the user — ssh does not say — but it can name
// the connection it tried, the user included, and put the credential first among the causes.
func TestCloudAPIWithTheWrongSSHUser(t *testing.T) {
	assertPointsAtTheCredential := func(t *testing.T, err error) {
		t.Helper()

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)

		// The user that was tried. Its absence is what made this hard to see.
		assert.Contains(t, failure.Checked, "ubuntu@10.0.0.5")
		assert.Contains(t, failure.Fix, "--ssh-user")
		assert.NotContains(t, failure.Fix, "AllowTcpForwarding",
			"sshd's forwarding settings are about a session that was never opened")
	}

	t.Run("the legacy backend gives only an exit status", func(t *testing.T) {
		// The reported shape: opening the forward is where ssh gave up, with exit 255 and
		// nothing else to go on.
		_, meta := cloudAPIServer(t, func(http.ResponseWriter, *http.Request) {}, "", nil)

		client := newFakeSSHClient("127.0.0.1:1")
		client.tunnelErr = fmt.Errorf("cannot open tunnel 'L:22323:z01-api.os.linx.ru:5000': %w", exitError(t, 255))
		check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

		_, err := check.Run(context.Background())

		assertPointsAtTheCredential(t, err)
	})

	t.Run("the default backend says so in words", func(t *testing.T) {
		// gossh reports what x/crypto produced, and then there is nothing to retry.
		_, meta := cloudAPIServer(t, func(http.ResponseWriter, *http.Request) {}, "", nil)

		client := newFakeSSHClient("127.0.0.1:1")
		client.tunnelErr = errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]")
		check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

		_, err := check.Run(context.Background())

		assertPointsAtTheCredential(t, err)
		assert.True(t, isPermanent(err), "a rejected key is not going to be accepted on a retry")
	})
}

// TestCloudAPIWhenTheForwardIsRefused: sshd will not open the port. The advice belongs on sshd,
// not on the cloud API.
func TestCloudAPIWhenTheForwardIsRefused(t *testing.T) {
	_, meta := cloudAPIServer(t, func(http.ResponseWriter, *http.Request) {}, "", nil)

	client := newFakeSSHClient("127.0.0.1:1")
	client.tunnelErr = errors.New("ssh: tcpip-forward request denied by peer")
	check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

	_, err := check.Run(context.Background())

	require.Error(t, err)
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Checked, "ssh port forward")
	assert.Contains(t, failure.Fix, "AllowTcpForwarding")
}

// TestCloudAPIWithACertificateItDoesNotTrust: the node reaches the API, but the CA in the
// configuration is not the one that signed it. That is a configuration mistake with a name, and it
// used to be reported as "could not reach Cloud API from master node".
func TestCloudAPIWithACertificateItDoesNotTrust(t *testing.T) {
	requireTunnelPortFree(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	providerJSON, err := json.Marshal(map[string]any{
		"server": "https://" + cloudAPIHost,
		// A valid PEM, but a different authority's: the handshake fails verification
		// rather than failing to parse, which is the mistake an operator actually makes.
		"caBundle": base64.StdEncoding.EncodeToString([]byte(unrelatedCA(t))),
	})
	require.NoError(t, err)

	meta := metaConfigForProvider(t, "vcd", string(providerJSON))
	client := newFakeSSHClient(server.Listener.Addr().String())
	check := CloudAPICheck{MetaConfig: meta, SSHProviderInitializer: sourceOf(client)}

	_, checkErr := check.Run(context.Background())

	require.Error(t, checkErr)
	var failure *preflight.Failure
	require.ErrorAs(t, checkErr, &failure)
	assert.Contains(t, failure.Observed, "TLS verification failed")

	// No number of retries makes an untrusted certificate trusted.
	var permanent *backoff.PermanentError
	assert.ErrorAs(t, checkErr, &permanent)
}

// unrelatedCA is a self-signed certificate that signed nothing in this test. Every httptest TLS
// server shares one built-in certificate, so "the CA of some other server" cannot be used to stand
// in for the wrong CA — it is the same one.
func unrelatedCA(t *testing.T) string {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "an authority that signed nothing here"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
