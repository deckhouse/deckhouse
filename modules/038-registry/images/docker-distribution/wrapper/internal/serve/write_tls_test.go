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

package serve

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-distribution/internal/config"
)

// client is a leaf the way the module's authority issues every one of them: for server AND client
// authentication — the signer's default profile — under the given name. That both usages are there
// is the reason the write endpoint checks names at all.
func (a authority) client(t *testing.T, commonName string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &key.PublicKey, a.key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

// pool is the authority as a client trusts it.
func (a authority) pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(a.cert)
	return pool
}

// writeFile puts content into a file of its own and returns the path.
func writeFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// writeSettings is the write endpoint's TLS material as the pod mounts it: a serving pair from the
// module's authority, and that same authority as the one client certificates must come from.
func writeSettings(t *testing.T, module authority, root string) (*configuration.Configuration, *config.Wrapper) {
	t.Helper()
	settings := storeConfiguration(root)
	// The serving listener's address, which the write endpoint only has to differ from.
	settings.HTTP.Addr = "127.0.0.1:4281"
	cert, key := module.issue(t, "127.0.0.1")
	settings.HTTP.TLS.Certificate = writeFile(t, "distribution.crt", cert)
	settings.HTTP.TLS.Key = writeFile(t, "distribution.key", key)

	wrapper := &config.Wrapper{
		Scope: "system/deckhouse",
		WriteEndpoint: config.WriteEndpoint{
			Address: "127.0.0.1:0",
			ClientCertCA: writeFile(t, "ingress-client-ca.crt",
				pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: module.cert.Raw})),
		},
	}
	return settings, wrapper
}

// presenting is an HTTP client that trusts the module's authority and presents certificate, if any.
func presenting(module authority, certificate *tls.Certificate) *http.Transport {
	settings := &tls.Config{RootCAs: module.pool(), MinVersion: tls.VersionTLS12}
	if certificate != nil {
		settings.Certificates = []tls.Certificate{*certificate}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = settings
	return transport
}

// TestTheWriteEndpointAdmitsItsTwoClientsAndBelievesOnlyTheIngress is the boundary of the listener that
// takes pushes on the node address of every master.
//
// The listener used to ask for a client certificate and serve whoever did not present one, which made
// it reachable around the ingress — and around `whitelistSourceRanges`, which only the ingress applies —
// by anybody who could reach the node. It now admits exactly two clients: the ingress controller, whose
// header names the operator behind it, and the syncer filling its own store, whose header is nobody's.
// Both hold a leaf of the same authority, and so does everything else in the storage pod.
//
// Run through writeTLS and writeHandler, the same two the production listener is built from, with a
// recorder in place of distribution.
func TestTheWriteEndpointAdmitsItsTwoClientsAndBelievesOnlyTheIngress(t *testing.T) {
	module := newAuthority(t, "registry-storage-ca")
	elsewhere := newAuthority(t, "somebody else")
	settings, wrapper := writeSettings(t, module, t.TempDir())

	tlsConfig, err := writeTLS(settings, wrapper)
	require.NoError(t, err)

	var observed string
	handler, err := writeHandler(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		observed = request.RemoteAddr
	}), settings, wrapper, unbounded(t, t.TempDir()))
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(handler)
	server.TLS = tlsConfig
	server.StartTLS()
	t.Cleanup(server.Close)

	ask := func(t *testing.T, certificate *tls.Certificate) (string, error) {
		t.Helper()
		observed = ""
		request, err := http.NewRequest(http.MethodGet, server.URL+"/v2/", http.NoBody)
		require.NoError(t, err)
		request.Header.Set("X-Forwarded-For", "203.0.113.7")
		response, err := (&http.Client{Transport: presenting(module, certificate)}).Do(request)
		if err != nil {
			return "", err
		}
		_ = response.Body.Close()
		return observed, nil
	}

	t.Run("no certificate", func(t *testing.T) {
		// What every client of the old listener was, and what a push straight to NODE_IP:4283
		// still is. Refused in the handshake, before the credentials are even asked for.
		_, err := ask(t, nil)
		require.Error(t, err, "a client without a certificate must not reach the store")
	})

	t.Run("the ingress's name from another authority", func(t *testing.T) {
		certificate := elsewhere.client(t, IngressClientName)
		_, err := ask(t, &certificate)
		require.Error(t, err, "a name anybody can put on a certificate proves nothing")
	})

	t.Run("the module's authority, issued to somebody else", func(t *testing.T) {
		// The token service's certificate: the same authority, client authentication among its
		// usages, mounted in the same pod. Not a client of this listener.
		certificate := module.client(t, "registry-storage-auth")
		_, err := ask(t, &certificate)
		require.Error(t, err, "the authority signs every leaf for client authentication; the name decides")
	})

	t.Run("the module's authority, issued to no name at all", func(t *testing.T) {
		// An empty name is not a wildcard: it matches neither client.
		certificate := module.client(t, "")
		_, err := ask(t, &certificate)
		require.Error(t, err, "a leaf without a name is nobody this listener admits")
		// And refused for its name, not for something incidental to how it was built.
		assert.ErrorContains(t, admitWriters(tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{certificate.Leaf},
		}), "does not admit")
	})

	t.Run("somebody else's leaf with the ingress's certificate appended", func(t *testing.T) {
		// The peer proves it holds the key for the first certificate only; the rest is a chain it
		// chose to send. Appending the genuine ingress certificate — public, and on the ingress
		// controller's disk — must not lend its name to the leaf in front of it.
		certificate := module.client(t, "registry-storage-auth")
		ingress := module.client(t, IngressClientName)
		certificate.Certificate = append(certificate.Certificate, ingress.Certificate[0])
		_, err := ask(t, &certificate)
		require.Error(t, err, "admission is decided by the leaf, not by anything after it in the chain")
		assert.ErrorContains(t, admitWriters(tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{certificate.Leaf, ingress.Leaf},
		}), "registry-storage-auth", "the leaf's name is the one checked")
	})

	t.Run("the ingress", func(t *testing.T) {
		certificate := module.client(t, IngressClientName)
		address, err := ask(t, &certificate)
		require.NoError(t, err, "`d8 mirror push` arrives through the ingress, and must keep working")
		assert.Equal(t, "203.0.113.7:0", address, "behind the ingress the header is the client's address")
	})

	t.Run("the syncer", func(t *testing.T) {
		certificate := module.client(t, SyncerClientName)
		address, err := ask(t, &certificate)
		require.NoError(t, err, "the fill, the replication and the collection write through this listener")
		assert.NotContains(t, address, "203.0.113.7", "the syncer fronts nobody; its header is a claim about itself")
		host, _, err := net.SplitHostPort(address)
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1", host)
	})
}

// TestTheSyncerPushesThroughTheWriteEndpointWithItsCertificate runs the listener writeEndpoint builds,
// in front of a real store, and pushes the way the syncer does: with the storage's own certificate.
// Without it the same push is refused — which is what a fill would get if any path of the syncer
// forgot to present it.
func TestTheSyncerPushesThroughTheWriteEndpointWithItsCertificate(t *testing.T) {
	module := newAuthority(t, "registry-storage-ca")
	root := t.TempDir()
	settings, wrapper := writeSettings(t, module, root)

	server, listener, err := writeEndpoint(context.Background(), settings, wrapper, unbounded(t, root), quiet())
	require.NoError(t, err)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	image, err := random.Image(1024, 1)
	require.NoError(t, err)
	tag, err := name.NewTag(listener.Addr().String() + "/system/deckhouse/filled:v1")
	require.NoError(t, err)

	err = remote.Write(tag, image, remote.WithTransport(presenting(module, nil)))
	require.Error(t, err, "a push without a client certificate is refused at the node address")

	syncer := module.client(t, SyncerClientName)
	require.NoError(t, remote.Write(tag, image, remote.WithTransport(presenting(module, &syncer))),
		"the syncer's own certificate admits its writes")
}

// TestTheWriteEndpointDoesNotServeTLSWithoutAClientAuthority: without an authority there is nothing to
// require, and a write endpoint that serves TLS to anybody is exactly what requiring one replaced.
func TestTheWriteEndpointDoesNotServeTLSWithoutAClientAuthority(t *testing.T) {
	module := newAuthority(t, "registry-storage-ca")
	settings, wrapper := writeSettings(t, module, t.TempDir())
	wrapper.WriteEndpoint.ClientCertCA = ""

	_, err := writeTLS(settings, wrapper)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clientCertCA")

	_, _, err = writeEndpoint(context.Background(), settings, wrapper, unbounded(t, t.TempDir()), quiet())
	require.Error(t, err, "and the endpoint does not open")
}
