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

package pki

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registrypki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

func testHosts() Hosts {
	return Hosts{
		Service:    "registry.d8-system.svc",
		Additional: []string{"10.0.0.1", "10.0.0.2"},
	}
}

func decode(t *testing.T, certificate CertKey) *x509.Certificate {
	t.Helper()

	parsed, err := registrypki.DecodeCertificate([]byte(certificate.Cert))
	require.NoError(t, err)
	return parsed
}

func TestGenerate(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	assert.True(t, state.Complete())

	// Every leaf must chain to the authority; otherwise nothing that trusts the
	// authority can verify the storage.
	authority := decode(t, state.CA)
	for name, certificate := range map[string]CertKey{
		"distribution":   state.Distribution,
		"auth":           state.Auth,
		"token":          state.Token,
		"ingress client": state.IngressClient,
	} {
		t.Run(name, func(t *testing.T) {
			leaf := decode(t, certificate)
			assert.NoError(t, registrypki.ValidateCertWithCAChain(leaf, authority))
		})
	}
}

// TestGenerateCoversEveryAddress matters because a node whose address is not in the
// certificate makes every agent on it fail to verify the storage, with no error
// anywhere near the thing that changed.
func TestGenerateCoversEveryAddress(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	serving := decode(t, state.Distribution)

	assert.Contains(t, serving.DNSNames, "registry.d8-system.svc")
	// The syncer reaches its own replica over the loopback interface.
	assert.Contains(t, serving.DNSNames, "localhost")

	addresses := make([]string, 0, len(serving.IPAddresses))
	for _, ip := range serving.IPAddresses {
		addresses = append(addresses, ip.String())
	}
	assert.Contains(t, addresses, "127.0.0.1")
	// The storage publishes a host port so a node can pull before the cluster network
	// is up, which means the node addresses have to be covered too.
	assert.Contains(t, addresses, "10.0.0.1")
	assert.Contains(t, addresses, "10.0.0.2")
}

// TestGenerateSeparatesTheSigningKey guards the reason the token certificate exists
// at all: a leaked serving key must not also let anyone mint pull tokens.
func TestGenerateSeparatesTheSigningKey(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	assert.NotEqual(t, state.Distribution.Key, state.Token.Key)
	assert.NotEqual(t, state.Distribution.Key, state.Auth.Key)
	assert.NotEqual(t, state.CA.Key, state.Distribution.Key)
}

// TestGenerateSeparatesReadFromWrite is the property behind having two users: the
// write endpoint is reachable from outside the cluster, so a leak of what every node
// uses to pull must not grant the ability to replace images.
func TestGenerateSeparatesReadFromWrite(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	assert.Equal(t, UserRO, state.RO.Name)
	assert.Equal(t, UserRW, state.RW.Name)
	assert.NotEqual(t, state.RO.Password, state.RW.Password)
	assert.NotEqual(t, state.RO.PasswordHash, state.RW.PasswordHash)

	// The hash is what the token service compares against, so the plain password
	// must not be stored in its place.
	assert.NotEqual(t, state.RO.Password, state.RO.PasswordHash)
}

// TestTheWriteListenerClientsCarryClientAuthentication pins what the write listener
// relies on. It requires a client certificate of this authority, verified for client
// authentication, from two leaves: the ingress client, and the serving certificate the
// syncer presents when it writes to its own store. The second has no leaf of its own
// precisely because the signer's default profile gives every leaf client
// authentication; if that ever changes, every fill, replication and collection is
// refused in the handshake. Checked on a reissued serving certificate as well, which is
// what a cluster that gained a master runs on.
func TestTheWriteListenerClientsCarryClientAuthentication(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	reissued := state
	changed, err := reissued.EnsureHosts(Hosts{
		Service:    "registry.d8-system.svc",
		Additional: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
	})
	require.NoError(t, err)
	require.True(t, changed)

	authority := x509.NewCertPool()
	authority.AddCert(decode(t, state.CA))

	for name, leaf := range map[string]struct {
		certificate CertKey
		commonName  string
	}{
		"ingress client":         {state.IngressClient, CommonNameIngressClient},
		"syncer":                 {state.Distribution, CommonNameDistribution},
		"syncer after reissuing": {reissued.Distribution, CommonNameDistribution},
	} {
		t.Run(name, func(t *testing.T) {
			certificate := decode(t, leaf.certificate)
			assert.Equal(t, leaf.commonName, certificate.Subject.CommonName,
				"the write listener admits its clients by these names")
			_, err := certificate.Verify(x509.VerifyOptions{
				Roots:     authority,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			})
			assert.NoError(t, err, "verified as a client certificate, the way the handshake does")
		})
	}
}

func TestGenerateIsFresh(t *testing.T) {
	first, err := Generate(testHosts())
	require.NoError(t, err)
	second, err := Generate(testHosts())
	require.NoError(t, err)

	assert.NotEqual(t, first.CA.Key, second.CA.Key)
	assert.NotEqual(t, first.HTTPSecret, second.HTTPSecret)
	assert.NotEqual(t, first.RO.Password, second.RO.Password)
}

// TestEnsureHostsReissuesOnANewNode covers a master joining the cluster.
func TestEnsureHostsReissuesOnANewNode(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	originalCA := state.CA
	originalToken := state.Token
	originalSecret := state.HTTPSecret

	hosts := testHosts()
	hosts.Additional = append(hosts.Additional, "10.0.0.3")

	changed, err := state.EnsureHosts(hosts)
	require.NoError(t, err)
	assert.True(t, changed)

	addresses := make([]string, 0)
	for _, ip := range decode(t, state.Distribution).IPAddresses {
		addresses = append(addresses, ip.String())
	}
	assert.Contains(t, addresses, "10.0.0.3")

	// The authority stays, so nothing that already trusts it has to be told; and the
	// unrelated material is left alone, so a node joining does not invalidate every
	// token in flight.
	assert.Equal(t, originalCA, state.CA)
	assert.Equal(t, originalToken, state.Token)
	assert.Equal(t, originalSecret, state.HTTPSecret)

	// The reissued certificate still chains to the same authority.
	assert.NoError(t, registrypki.ValidateCertWithCAChain(
		decode(t, state.Distribution), decode(t, state.CA)))
}

// TestEnsureHostsIsIdempotent keeps an unchanged node list from reissuing on every
// pass, which would restart the storage every time the hook runs.
func TestEnsureHostsIsIdempotent(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)
	before := state.Distribution

	changed, err := state.EnsureHosts(testHosts())
	require.NoError(t, err)

	assert.False(t, changed)
	assert.Equal(t, before, state.Distribution)
}

// TestEnsureHostsIgnoresOrder matters because the node list comes from the API in
// no guaranteed order, and reissuing on a reordering would be a restart loop.
func TestEnsureHostsIgnoresOrder(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	reordered := testHosts()
	reordered.Additional = []string{"10.0.0.2", "10.0.0.1"}

	changed, err := state.EnsureHosts(reordered)
	require.NoError(t, err)
	assert.False(t, changed)
}

// TestEnsureHostsDoesNotShrink documents the current behaviour: a removed node's
// address stays in the certificate until something else forces a reissue. Covering
// an address that is gone is harmless, while reissuing on every node removal is a
// restart of the whole storage tier.
func TestEnsureHostsDoesNotShrink(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	fewer := testHosts()
	fewer.Additional = []string{"10.0.0.1"}

	changed, err := state.EnsureHosts(fewer)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestComplete(t *testing.T) {
	full, err := Generate(testHosts())
	require.NoError(t, err)

	tests := map[string]func(*State){
		"no authority":      func(s *State) { s.CA = CertKey{} },
		"no serving key":    func(s *State) { s.Distribution.Key = "" },
		"no token":          func(s *State) { s.Token = CertKey{} },
		"no ingress client": func(s *State) { s.IngressClient = CertKey{} },
		"no HTTP secret":    func(s *State) { s.HTTPSecret = "" },
		"no read user":      func(s *State) { s.RO = User{} },
		"no write password": func(s *State) { s.RW.Password = "" },
		"no password hash":  func(s *State) { s.RW.PasswordHash = "" },
	}

	for name, break_ := range tests {
		t.Run(name, func(t *testing.T) {
			broken := full
			break_(&broken)

			// A partial state is worse than an empty one: the empty one is regenerated,
			// while the partial one reaches the storage and fails at handshake time with
			// nothing pointing at the missing piece.
			assert.False(t, broken.Complete())
		})
	}

	var absent *State
	assert.False(t, absent.Complete())
}

func TestCertificateCovers(t *testing.T) {
	state, err := Generate(Hosts{Service: "registry.d8-system.svc", Additional: []string{"10.0.0.1"}})
	require.NoError(t, err)

	covered, err := certificateCovers(state.Distribution.Cert,
		[]string{"registry.d8-system.svc", "127.0.0.1", "10.0.0.1"})
	require.NoError(t, err)
	assert.True(t, covered)

	covered, err = certificateCovers(state.Distribution.Cert, []string{"10.9.9.9"})
	require.NoError(t, err)
	assert.False(t, covered)

	covered, err = certificateCovers(state.Distribution.Cert, []string{"other.example.com"})
	require.NoError(t, err)
	assert.False(t, covered)

	covered, err = certificateCovers("", []string{"anything"})
	require.NoError(t, err)
	assert.False(t, covered, "an absent certificate covers nothing")

	_, err = certificateCovers("not a certificate", []string{"anything"})
	assert.Error(t, err)
}

// requirePKCS8 asserts what OpenSSL needs to load a key labelled "PRIVATE KEY": one
// PKCS#8 block, read here with the strict parser rather than the lenient ones the
// module's Go readers use. Returns the key, after checking it is the certificate's.
func requirePKCS8(t *testing.T, pair CertKey) crypto.Signer {
	t.Helper()

	block, rest := pem.Decode([]byte(pair.Key))
	require.NotNil(t, block)
	assert.Empty(t, rest)
	assert.Equal(t, "PRIVATE KEY", block.Type)

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err, "OpenSSL reads this label as PKCS#8 and nothing else")
	key, ok := parsed.(crypto.Signer)
	require.True(t, ok)

	matches, err := registrypki.ComparePublicKeys(decode(t, pair).PublicKey, key.Public())
	require.NoError(t, err)
	assert.True(t, matches, "the key is the certificate's")
	return key
}

// The ingress controller loads the ingress client key with OpenSSL, even with
// proxy-ssl-verify off, and one key it cannot read fails its whole configuration.
func TestGenerateWritesTheIngressClientKeyInPKCS8(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	requirePKCS8(t, state.IngressClient)
}

// TestEnsureIngressClientKeyHealsAStoredState is a cluster generated before the key
// was written in PKCS#8: the next run re-encodes the same key and changes nothing
// else, so no authority is regenerated and no node has to be told anything.
func TestEnsureIngressClientKeyHealsAStoredState(t *testing.T) {
	state, err := Generate(testHosts())
	require.NoError(t, err)

	// What the shared encoder wrote: PKCS#1 under the PKCS#8 label.
	original, err := registrypki.DecodePrivateKey([]byte(state.IngressClient.Key))
	require.NoError(t, err)
	legacy, err := registrypki.EncodePrivateKey(original)
	require.NoError(t, err)
	block, _ := pem.Decode(legacy)
	require.NotNil(t, block)
	require.Equal(t, "PRIVATE KEY", block.Type)
	_, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	require.Error(t, err, "the stored key this test starts from must be the one OpenSSL refuses")

	state.IngressClient.Key = string(legacy)
	before := state

	changed, err := state.EnsureIngressClientKey()
	require.NoError(t, err)
	assert.True(t, changed)

	key := requirePKCS8(t, state.IngressClient)
	assert.True(t, original.(interface{ Equal(crypto.PrivateKey) bool }).Equal(key),
		"re-encoded, not reissued")

	assert.True(t, state.Complete())
	assert.Equal(t, before.IngressClient.Cert, state.IngressClient.Cert)
	before.IngressClient.Key = state.IngressClient.Key
	assert.Equal(t, before, state, "nothing but the ingress client key changes")

	// And only once: an encoding that changed on every run would rewrite the secret
	// on every run.
	healed := state.IngressClient.Key
	changed, err = state.EnsureIngressClientKey()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, healed, state.IngressClient.Key)
}

// A pair that cannot be re-encoded — a key that is not the certificate's, or one
// that does not decode — is reissued from the same authority. Written out as it is,
// it would fail the controller's configuration; refused, it failed the hook on every
// run with nothing that would ever change it.
func TestEnsureIngressClientKeyReissuesAPairThatDoesNotMatch(t *testing.T) {
	for name, spoil := range map[string]func(*State){
		"a key of another certificate": func(s *State) { s.IngressClient.Key = s.Distribution.Key },
		"a key that does not decode":   func(s *State) { s.IngressClient.Key = "not a key" },
		"a certificate that does not decode": func(s *State) {
			s.IngressClient.Cert = "not a certificate"
		},
	} {
		t.Run(name, func(t *testing.T) {
			state, err := Generate(testHosts())
			require.NoError(t, err)
			spoil(&state)
			before := state

			changed, err := state.EnsureIngressClientKey()
			require.NoError(t, err)
			assert.True(t, changed)

			requirePKCS8(t, state.IngressClient)
			reissued := decode(t, state.IngressClient)
			assert.Equal(t, CommonNameIngressClient, reissued.Subject.CommonName)
			require.NoError(t, reissued.CheckSignatureFrom(decode(t, state.CA)),
				"issued by the authority the storage already trusts")

			before.IngressClient = state.IngressClient
			assert.Equal(t, before, state, "nothing but the ingress client pair changes")

			changed, err = state.EnsureIngressClientKey()
			require.NoError(t, err)
			assert.False(t, changed, "and only once")
		})
	}
}
