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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authority signs serving certificates the way the module's storage PKI does.
type authority struct {
	cert *x509.Certificate
	key  crypto.Signer
}

func newAuthority(t *testing.T, name string) authority {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return authority{cert: cert, key: key}
}

// issue is a serving certificate for the addresses, with its key, both PEM-encoded.
func (a authority) issue(t *testing.T, addresses ...string) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "registry-storage"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	for _, address := range addresses {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(address))
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &key.PublicKey, a.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// watch starts a watcher over freshly written files and returns where they are and a channel closed
// when it decides to restart.
func watch(t *testing.T, self string, cert, key []byte) (string, string, <-chan struct{}) {
	t.Helper()
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "registry.crt"), filepath.Join(dir, "registry.key")
	require.NoError(t, os.WriteFile(certPath, cert, 0o600))
	require.NoError(t, os.WriteFile(keyPath, key, 0o600))

	changed := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go watchCertificate(ctx, certPath, keyPath, self, 10*time.Millisecond, quietLog(), func() { close(changed) })
	return certPath, keyPath, changed
}

func restarted(changed <-chan struct{}, within time.Duration) bool {
	select {
	case <-changed:
		return true
	case <-time.After(within):
		return false
	}
}

// TestTheReplicaTheCertificateMissedRestarts: the replica on the master that joined last started before
// the certificate naming that master did, and went on serving one valid for the others only. Once the
// lease went to it nobody could replicate from it. A reissued pair is noticed — and a half-written one
// is not, because restarting onto it would leave the listener serving nothing.
func TestTheReplicaTheCertificateMissedRestarts(t *testing.T) {
	ca := newAuthority(t, "registry-storage-ca")
	cert, key := ca.issue(t, "127.0.0.1", "10.12.0.148", "10.12.3.12")
	certPath, keyPath, changed := watch(t, "10.12.2.36", cert, key)

	reissued, reissuedKey := ca.issue(t, "127.0.0.1", "10.12.0.148", "10.12.2.36", "10.12.3.12")
	require.NoError(t, os.WriteFile(certPath, reissued, 0o600))
	assert.False(t, restarted(changed, 100*time.Millisecond), "the certificate alone, before its key, is not a pair")

	require.NoError(t, os.WriteFile(keyPath, reissuedKey, 0o600))
	assert.True(t, restarted(changed, 5*time.Second), "a reissued certificate was not noticed")
}

// TestAReplicaTheCertificateCoversKeepsServing: every replica restarting for a master that joined took
// the leader down for a few seconds — every pull in an air-gapped cluster failing for those seconds.
// The others are covered by what they serve, under the same authority, and do not need the new one.
func TestAReplicaTheCertificateCoversKeepsServing(t *testing.T) {
	ca := newAuthority(t, "registry-storage-ca")
	cert, key := ca.issue(t, "127.0.0.1", "10.12.0.148", "10.12.3.12")
	certPath, keyPath, changed := watch(t, "10.12.0.148", cert, key)

	reissued, reissuedKey := ca.issue(t, "127.0.0.1", "10.12.0.148", "10.12.2.36", "10.12.3.12")
	require.NoError(t, os.WriteFile(certPath, reissued, 0o600))
	require.NoError(t, os.WriteFile(keyPath, reissuedKey, 0o600))
	assert.False(t, restarted(changed, 200*time.Millisecond))
}

// TestAnotherAuthorityAlwaysRestarts: clients verify against the authority, so a certificate from a new
// one is what every replica has to serve.
func TestAnotherAuthorityAlwaysRestarts(t *testing.T) {
	cert, key := newAuthority(t, "registry-storage-ca").issue(t, "127.0.0.1", "10.12.0.148")
	certPath, keyPath, changed := watch(t, "10.12.0.148", cert, key)

	reissued, reissuedKey := newAuthority(t, "registry-storage-ca").issue(t, "127.0.0.1", "10.12.0.148")
	require.NoError(t, os.WriteFile(certPath, reissued, 0o600))
	require.NoError(t, os.WriteFile(keyPath, reissuedKey, 0o600))
	assert.True(t, restarted(changed, 5*time.Second))
}

// TestWhatCannotBeToldRestarts: an address that is not a replica's own says nothing about what it
// serves, and restarting is never wrong, only sometimes unnecessary.
func TestWhatCannotBeToldRestarts(t *testing.T) {
	ca := newAuthority(t, "registry-storage-ca")
	served, _ := ca.issue(t, "127.0.0.1", "10.12.0.148")
	reissued, _ := ca.issue(t, "127.0.0.1", "10.12.0.148", "10.12.2.36")

	assert.True(t, needsReissued(served, reissued, ""))
	assert.True(t, needsReissued(served, reissued, "0.0.0.0"))
	assert.True(t, needsReissued(served, reissued, "registry.d8-system.svc"))
	assert.True(t, needsReissued([]byte("not a certificate"), reissued, "10.12.0.148"))
	assert.False(t, needsReissued(served, reissued, "10.12.0.148"))
}

// TestAnUnchangedCertificateIsLeftAlone: nothing restarts while the files stay as they were.
func TestAnUnchangedCertificateIsLeftAlone(t *testing.T) {
	cert, key := newAuthority(t, "registry-storage-ca").issue(t, "127.0.0.1")
	_, _, changed := watch(t, "127.0.0.1", cert, key)
	assert.False(t, restarted(changed, 200*time.Millisecond))
}
