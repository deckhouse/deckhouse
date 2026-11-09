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

package run

import (
	"context"
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
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

// moduleAuthority stands in for the storage PKI: one authority, and leaves for server and client
// authentication alike, the way the hook issues every one of them.
type moduleAuthority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newModuleAuthority(t *testing.T) moduleAuthority {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "registry-storage-ca"},
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
	return moduleAuthority{cert: cert, key: key}
}

func (a moduleAuthority) pem() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.cert.Raw}))
}

func (a moduleAuthority) issue(t *testing.T, commonName string) ([]byte, []byte) {
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
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// writeListener serves store over TLS the way the write endpoint does: a client without a certificate
// of the authority is refused in the handshake. Returns its address.
func writeListener(t *testing.T, authority moduleAuthority, store http.Handler) string {
	t.Helper()
	certPEM, keyPEM := authority.issue(t, "registry-storage")
	serving, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	clients := x509.NewCertPool()
	clients.AddCert(authority.cert)

	server := httptest.NewUnstartedServer(store)
	// The loop's wait for its registry dials the port without a handshake, which the server would
	// log as an error on every pass.
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serving},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clients,
		MinVersion:   tls.VersionTLS12,
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	return parsed.Host
}

// syncerCertificate writes the pair where the loop reads it from, as the PKI mount holds the serving
// certificate the syncer presents.
func syncerCertificate(t *testing.T, authority moduleAuthority) *fill.ClientCertificate {
	t.Helper()
	certPEM, keyPEM := authority.issue(t, "registry-storage")
	dir := t.TempDir()
	certificate := &fill.ClientCertificate{
		CertFile: filepath.Join(dir, "distribution.crt"),
		KeyFile:  filepath.Join(dir, "distribution.key"),
	}
	require.NoError(t, os.WriteFile(certificate.CertFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(certificate.KeyFile, keyPEM, 0o600))
	return certificate
}

// TestEveryWriteToItsOwnStorePresentsTheClientCertificate is the syncer's side of the write endpoint
// requiring a client certificate.
//
// The endpoint refuses a handshake without one, and the syncer writes to it on three paths: the fill
// from an upstream, the replication from the leader — both through writeRegistry — and the garbage
// collection, which is handed LocalOptions instead. A path that forgot the certificate would not fail
// loudly: a fill reports references it could not copy, a collection reports warnings. So each is
// driven here against a listener that requires one.
func TestEveryWriteToItsOwnStorePresentsTheClientCertificate(t *testing.T) {
	authority := newModuleAuthority(t)

	// One store behind two listeners, as in the pod: the write endpoint, which requires the
	// certificate, and a plain one this test reads the store through.
	store := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	writeEndpoint := writeListener(t, authority, store)
	inspect := httptest.NewServer(store)
	t.Cleanup(inspect.Close)
	inspected, err := url.Parse(inspect.URL)
	require.NoError(t, err)

	upstream := startRegistry(t)
	serving := startRegistry(t)

	one := pushDigest(t, upstream, "deckhouse/ee")
	two := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one, two})

	loopFor := func(t *testing.T, certificate *fill.ClientCertificate) *Loop {
		loop, _, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
			Upstream: &registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
				},
			},
			Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
			NeedSync: true,
		}), deployedRelease("v1.70.1"))
		loop.LocalAddress = serving
		loop.WriteAddress = writeEndpoint
		loop.ReportedAddress = serving
		loop.LocalCA = authority.pem()
		loop.LocalClientCertificate = certificate
		return loop
	}

	push := func(t *testing.T, options []remote.Option) error {
		t.Helper()
		image, err := random.Image(256, 1)
		require.NoError(t, err)
		tag, err := name.NewTag(writeEndpoint + "/system/deckhouse/collected:v1")
		require.NoError(t, err)
		return remote.Write(tag, image, options...)
	}

	t.Run("the fill writes through writeRegistry", func(t *testing.T) {
		loop := loopFor(t, syncerCertificate(t, authority))
		require.NoError(t, loop.once(context.Background()))

		assert.True(t, holdsDigest(t, inspected.Host, "system/deckhouse", one),
			"the fill did not get through the write endpoint")
		assert.True(t, holdsDigest(t, inspected.Host, "system/deckhouse", two))
	})

	t.Run("the replication's destination is the same client", func(t *testing.T) {
		loop := loopFor(t, syncerCertificate(t, authority))
		destination, err := loop.writeRegistry()
		require.NoError(t, err)
		assert.Equal(t, writeEndpoint, destination.Address)
		require.NoError(t, push(t, destination.Options))
	})

	t.Run("the garbage collection is handed LocalOptions", func(t *testing.T) {
		loop := loopFor(t, syncerCertificate(t, authority))
		require.NoError(t, push(t, loop.LocalOptions()),
			"a collection without the certificate deletes nothing and reports only warnings")
	})

	t.Run("without the certificate every one of them is refused", func(t *testing.T) {
		loop := loopFor(t, nil)
		destination, err := loop.writeRegistry()
		require.NoError(t, err)
		require.Error(t, push(t, destination.Options))
		require.Error(t, push(t, loop.LocalOptions()))
	})
}

// TestAProbeOfTheWriteEndpointPresentsTheClientCertificate: a client of this replica's registry that
// speaks plain HTTP rather than through the registry options — a probe asking whether the listeners
// answer — has to be admitted to the write endpoint as the copy is. Built on localTransport it is; built
// on a transport that only trusts the authority, it is refused at the handshake, and a probe would read
// that as the registry not answering on every replica at once.
func TestAProbeOfTheWriteEndpointPresentsTheClientCertificate(t *testing.T) {
	authority := newModuleAuthority(t)
	writeEndpoint := writeListener(t, authority, registry.New(registry.Logger(log.New(io.Discard, "", 0))))

	probe := func(t *testing.T, loop *Loop) error {
		t.Helper()
		transport, err := loop.localTransport()
		require.NoError(t, err)
		defer transport.CloseIdleConnections()
		response, err := (&http.Client{Transport: transport}).Get("https://" + writeEndpoint + "/v2/")
		if err != nil {
			return err
		}
		return response.Body.Close()
	}

	loop, _, _ := newLoop(t, true, nil)
	loop.LocalCA = authority.pem()
	loop.LocalClientCertificate = syncerCertificate(t, authority)
	require.NoError(t, probe(t, loop))

	loop.LocalClientCertificate = nil
	err := probe(t, loop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate required")
}
