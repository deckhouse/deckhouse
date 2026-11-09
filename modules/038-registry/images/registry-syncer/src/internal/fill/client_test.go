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

package fill

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAuthority issues leaves the way the module's authority does: every one for server and client
// authentication alike.
type testAuthority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestAuthority(t *testing.T, commonName string) testAuthority {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return testAuthority{cert: cert, key: key}
}

func (a testAuthority) pem() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.cert.Raw}))
}

func (a testAuthority) pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(a.cert)
	return pool
}

// leaf issues a certificate valid for the loopback, and returns it with its key in PEM.
func (a testAuthority) leaf(t *testing.T, commonName string) (tls.Certificate, []byte, []byte) {
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
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &key.PublicKey, a.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair, certPEM, keyPEM
}

// clientFiles writes a leaf of the authority where a ClientCertificate reads it from, the way the
// PKI mount holds distribution.crt and distribution.key.
func (a testAuthority) clientFiles(t *testing.T, commonName string) *ClientCertificate {
	t.Helper()
	_, certPEM, keyPEM := a.leaf(t, commonName)
	dir := t.TempDir()
	client := &ClientCertificate{
		CertFile: filepath.Join(dir, "distribution.crt"),
		KeyFile:  filepath.Join(dir, "distribution.key"),
	}
	require.NoError(t, os.WriteFile(client.CertFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(client.KeyFile, keyPEM, 0o600))
	return client
}

// listener runs handler over TLS with the authority's serving certificate and the given client
// authentication, and records the common name of every client certificate it was shown.
type listener struct {
	address string

	mu    sync.Mutex
	shown []string
}

func (l *listener) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.shown...)
}

func listen(
	t *testing.T, server testAuthority, handler http.Handler, mode tls.ClientAuthType, clients *x509.CertPool,
) *listener {
	t.Helper()
	serving, _, _ := server.leaf(t, "registry-storage")
	result := &listener{}
	recording := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.TLS != nil && len(request.TLS.PeerCertificates) > 0 {
			result.mu.Lock()
			result.shown = append(result.shown, request.TLS.PeerCertificates[0].Subject.CommonName)
			result.mu.Unlock()
		}
		handler.ServeHTTP(writer, request)
	})
	httpServer := httptest.NewUnstartedServer(recording)
	httpServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{serving},
		ClientAuth:   mode,
		ClientCAs:    clients,
		MinVersion:   tls.VersionTLS12,
	}
	httpServer.StartTLS()
	t.Cleanup(httpServer.Close)

	parsed, err := url.Parse(httpServer.URL)
	require.NoError(t, err)
	result.address = parsed.Host
	return result
}

func quietRegistry() http.Handler {
	return registry.New(registry.Logger(log.New(io.Discard, "", 0)))
}

func pushWith(t *testing.T, address string, options []remote.Option) error {
	t.Helper()
	image, err := random.Image(256, 1)
	require.NoError(t, err)
	tag, err := name.NewTag(address + "/system/deckhouse/filled:v1")
	require.NoError(t, err)
	return remote.Write(tag, image, options...)
}

// TestTheWriteEndpointTakesAPushOnlyWithTheClientCertificate is the listener the fill, the replication
// and the garbage collection write through: it refuses a handshake without a client certificate of the
// module's authority. The options built with the pair get through; the ones every other client of the
// syncer uses do not.
func TestTheWriteEndpointTakesAPushOnlyWithTheClientCertificate(t *testing.T) {
	module := newTestAuthority(t, "registry-storage-ca")
	endpoint := listen(t, module, quietRegistry(), tls.RequireAndVerifyClientCert, module.pool())

	without, err := RegistryOptions(module.pem(), "", "")
	require.NoError(t, err)
	require.Error(t, pushWith(t, endpoint.address, without),
		"without the pair the handshake is refused, and so is every write of the fill")

	with, err := ClientRegistryOptions(module.pem(), "", "", module.clientFiles(t, "registry-storage"))
	require.NoError(t, err)
	require.NoError(t, pushWith(t, endpoint.address, with))
	assert.Contains(t, endpoint.seen(), "registry-storage")
}

// TestTheClientCertificateGoesOnlyWhereItIsAskedFor: the same options reach the serving listener, which
// does not ask for one, and a registry may ask for certificates of an authority this is not from. In
// neither case is the pair shown.
func TestTheClientCertificateGoesOnlyWhereItIsAskedFor(t *testing.T) {
	module := newTestAuthority(t, "registry-storage-ca")
	other := newTestAuthority(t, "somebody else")
	client := module.clientFiles(t, "registry-storage")

	options, err := ClientRegistryOptions(module.pem(), "", "", client)
	require.NoError(t, err)

	t.Run("a listener that does not ask", func(t *testing.T) {
		serving := listen(t, module, quietRegistry(), tls.NoClientCert, nil)
		require.NoError(t, pushWith(t, serving.address, options))
		assert.Empty(t, serving.seen())
	})

	t.Run("a listener that asks for another authority", func(t *testing.T) {
		elsewhere := listen(t, module, quietRegistry(), tls.RequestClientCert, other.pool())
		require.NoError(t, pushWith(t, elsewhere.address, options))
		assert.Empty(t, elsewhere.seen(), "a certificate the listener cannot verify is not offered to it")
	})
}

// TestTheClientCertificateIsReadWhenAsked: the files are a Secret mount the hook reissues when a master
// joins, and a transport that loaded them once would go on presenting the old pair. And a pair that
// cannot be read fails the handshake with the reason, rather than going out empty.
func TestTheClientCertificateIsReadWhenAsked(t *testing.T) {
	module := newTestAuthority(t, "registry-storage-ca")
	endpoint := listen(t, module, http.NotFoundHandler(), tls.RequireAndVerifyClientCert, module.pool())
	client := module.clientFiles(t, "registry-storage")

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: module.pool(), MinVersion: tls.VersionTLS12}
	client.Present(transport)
	httpClient := &http.Client{Transport: transport}

	get := func() error {
		response, err := httpClient.Get("https://" + endpoint.address + "/v2/")
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		transport.CloseIdleConnections()
		return nil
	}

	require.NoError(t, get())

	_, certPEM, keyPEM := module.leaf(t, "registry-storage-reissued")
	require.NoError(t, os.WriteFile(client.CertFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(client.KeyFile, keyPEM, 0o600))
	require.NoError(t, get())
	assert.Equal(t, []string{"registry-storage", "registry-storage-reissued"}, endpoint.seen(),
		"the next handshake presents the pair on disk now")

	require.NoError(t, os.Remove(client.KeyFile))
	err := get()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading the client certificate")
}

// TestNoClientCertificateLeavesTheTransportAsItWas: a loop pointed at a registry that does not ask has
// none to give, and the options must be exactly what they were.
func TestNoClientCertificateLeavesTheTransportAsItWas(t *testing.T) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	before := transport.TLSClientConfig
	var client *ClientCertificate
	client.Present(transport)
	assert.Same(t, before, transport.TLSClientConfig)

	options, err := ClientRegistryOptions("", "", "", nil)
	require.NoError(t, err)
	assert.Len(t, options, 1, "the transport and nothing else: no credentials were given either")
}

// TestAPairReadAcrossAReissueIsReadAgain: the certificate and the key are two reads, and kubelet can
// replace the mount between them. The certificate of the old issue with the key of the new one must not
// go out as a pair — the handshake would fail on a key that does not sign for the certificate — nor
// fail the handshake while the mount already holds a pair that matches.
func TestAPairReadAcrossAReissueIsReadAgain(t *testing.T) {
	module := newTestAuthority(t, "registry-storage-ca")
	_, oldCert, oldKey := module.leaf(t, "registry-storage")
	_, newCert, newKey := module.leaf(t, "registry-storage-reissued")

	t.Run("swapped between the two reads", func(t *testing.T) {
		reads := 0
		read := func(file string) ([]byte, error) {
			reads++
			switch {
			case reads == 1:
				return oldCert, nil // read before the swap
			case file == "key":
				return newKey, nil
			default:
				return newCert, nil
			}
		}

		pair, err := loadPair("cert", "key", read)
		require.NoError(t, err)
		require.NotNil(t, pair.Leaf)
		assert.Equal(t, "registry-storage-reissued", pair.Leaf.Subject.CommonName)
		assert.Equal(t, 4, reads, "read once more, both files")
	})

	t.Run("a mount that holds a mismatch", func(t *testing.T) {
		reads := 0
		read := func(file string) ([]byte, error) {
			reads++
			if file == "key" {
				return oldKey, nil
			}
			return newCert, nil
		}

		_, err := loadPair("cert", "key", read)
		require.Error(t, err, "a key of another certificate is never presented")
		assert.Equal(t, 2*pairReadAttempts, reads, "and is not read without end")
	})
}

// TestATransportOfItsOwnIsRefusedByTheWriteEndpoint is anything that speaks plain HTTP to the write
// endpoint rather than through the registry options, such as a probe of the listeners: built on
// ClientTransport it gets an answer, built on a transport that only trusts the authority it does not
// get past the handshake.
func TestATransportOfItsOwnIsRefusedByTheWriteEndpoint(t *testing.T) {
	module := newTestAuthority(t, "registry-storage-ca")
	endpoint := listen(t, module, quietRegistry(), tls.RequireAndVerifyClientCert, module.pool())

	get := func(transport *http.Transport) error {
		defer transport.CloseIdleConnections()
		response, err := (&http.Client{Transport: transport}).Get("https://" + endpoint.address + "/v2/")
		if err != nil {
			return err
		}
		return response.Body.Close()
	}

	with, err := ClientTransport(module.pem(), module.clientFiles(t, "registry-storage"))
	require.NoError(t, err)
	require.NoError(t, get(with))

	without, err := ClientTransport(module.pem(), nil)
	require.NoError(t, err)
	err = get(without)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate required")
}
