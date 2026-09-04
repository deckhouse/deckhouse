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

package realip

import (
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authority is a certificate authority and the certificates it signs, built in the test so that
// "trusted" and "from somewhere else" are the same code path apart from which authority signed.
type authority struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	pem         []byte
}

func newAuthority(t *testing.T, name string) *authority {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return &authority{
		certificate: certificate,
		key:         key,
		pem:         pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}
}

// issue signs a certificate for the given usage, which is the distinction the trust decision rests
// on: a server certificate from the right authority is not a licence to speak for somebody else's
// address.
func (a *authority) issue(t *testing.T, name string, usage x509.ExtKeyUsage) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, a.certificate, &key.PublicKey, a.key)
	require.NoError(t, err)

	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

func (a *authority) file(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(path, a.pem, 0o600))
	return path
}

// serve runs a TLS listener that asks for a client certificate but does not require one, which is
// how the write endpoint listens: a push without a certificate is still served, it just does not
// get to say where it came from.
func serve(t *testing.T, handler http.Handler, server tls.Certificate) *httptest.Server {
	t.Helper()

	listener := httptest.NewUnstartedServer(handler)
	listener.TLS = &tls.Config{
		Certificates: []tls.Certificate{server},
		ClientAuth:   tls.RequestClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	listener.StartTLS()
	t.Cleanup(listener.Close)
	return listener
}

func client(t *testing.T, ca *authority, certificate *tls.Certificate) *http.Client {
	t.Helper()

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(ca.pem))

	settings := &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	if certificate != nil {
		settings.Certificates = []tls.Certificate{*certificate}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = settings
	return &http.Client{Transport: transport}
}

// seen records what the registry behind this middleware would log and hand to the token service.
func seen(t *testing.T, ca *authority, clientCertCA string, certificate *tls.Certificate) string {
	t.Helper()

	var observed string
	handler, err := Handler(clientCertCA, http.HandlerFunc(
		func(_ http.ResponseWriter, request *http.Request) {
			observed = request.RemoteAddr
		}))
	require.NoError(t, err)

	server := serve(t, handler, ca.issue(t, "registry", x509.ExtKeyUsageServerAuth))

	request, err := http.NewRequest(http.MethodGet, server.URL+"/v2/", nil)
	require.NoError(t, err)
	request.Header.Set("X-Forwarded-For", "203.0.113.7")

	response, err := client(t, ca, certificate).Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	return observed
}

// TestTheForwardedAddressIsBelievedOnlyBehindTheRightCertificate is the boundary itself.
//
// The write endpoint is the one listener reachable from outside the cluster, and what the header
// decides is which address ends up in the registry's log and in what the token service is told. A
// header anybody may set is worth nothing; a header behind a certificate this module issued to the
// ingress is worth exactly as much as the ingress.
func TestTheForwardedAddressIsBelievedOnlyBehindTheRightCertificate(t *testing.T) {
	ingress := newAuthority(t, "ingress")
	elsewhere := newAuthority(t, "somebody else")

	trustedCertificate := ingress.issue(t, "ingress-controller", x509.ExtKeyUsageClientAuth)
	foreignCertificate := elsewhere.issue(t, "ingress-controller", x509.ExtKeyUsageClientAuth)
	serverCertificate := ingress.issue(t, "not-a-client", x509.ExtKeyUsageServerAuth)

	authorityFile := ingress.file(t)

	t.Run("the front this listener has", func(t *testing.T) {
		assert.Equal(t, "203.0.113.7:0", seen(t, ingress, authorityFile, &trustedCertificate),
			"behind the ingress the header is the client's address")
	})

	t.Run("a certificate from another authority", func(t *testing.T) {
		address := seen(t, ingress, authorityFile, &foreignCertificate)
		assert.NotContains(t, address, "203.0.113.7",
			"anybody can obtain a certificate somewhere; it does not make them the ingress")
		assert.Contains(t, address, "127.0.0.1")
	})

	t.Run("a server certificate from the right authority", func(t *testing.T) {
		address := seen(t, ingress, authorityFile, &serverCertificate)
		assert.NotContains(t, address, "203.0.113.7",
			"this module issues server certificates too, and they are not a licence to speak for a client")
	})

	t.Run("no certificate at all", func(t *testing.T) {
		address := seen(t, ingress, authorityFile, nil)
		assert.NotContains(t, address, "203.0.113.7")
		assert.Contains(t, address, "127.0.0.1", "the request is served, it just speaks for itself")
	})

	t.Run("no authority configured", func(t *testing.T) {
		// The serving listener: nothing fronts it, so nobody is entitled to speak for a client, and
		// a header is just a string a node put there. The previous implementation had two knobs for
		// this — one to enable the header, one to name the authority — and enabling it without an
		// authority believed everybody. There is one knob here, and its empty value is the safe
		// one.
		address := seen(t, ingress, "", nil)
		assert.NotContains(t, address, "203.0.113.7",
			"with no authority to check against, the claim is nobody's to make")
		assert.Contains(t, address, "127.0.0.1")
	})
}

func TestHandlerRefusesAnUnusableAuthority(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(broken, []byte("this is not a certificate"), 0o600))

	_, err := Handler(broken, http.NotFoundHandler())
	require.Error(t, err, "an authority that verifies nothing would silently trust nobody, forever")
	assert.Contains(t, err.Error(), "no certificate")
}
