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
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The whole point on an Engine node: nothing has written the material and the agent has
// to be able to serve anyway.
func TestEnsureGeneratesUsableMaterial(t *testing.T) {
	material := &OnDisk{Dir: filepath.Join(t.TempDir(), "pki")}

	require.NoError(t, material.Ensure())
	require.NoError(t, material.Ready(), "the agent would refuse to start")

	certificate, err := material.Certificate(nil)
	require.NoError(t, err)
	require.NotNil(t, certificate)

	authority, err := material.CA()
	require.NoError(t, err)

	// The runtime verifies what the agent serves against the authority beside it, so the
	// two have to be one pair — the failure the bashible step's staging directory exists
	// to prevent.
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(authority)), "the authority is not a PEM certificate")

	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	_, err = leaf.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
	require.NoError(t, err, "the certificate does not verify against the authority written beside it")
}

// The runtime dials the agent on the loopback address and on nothing else, so those are
// the names that have to be on the certificate.
func TestEnsureCertificateNamesTheLoopback(t *testing.T) {
	material := &OnDisk{Dir: filepath.Join(t.TempDir(), "pki")}
	require.NoError(t, material.Ensure())

	certificate, err := material.Certificate(nil)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)

	require.NoError(t, leaf.VerifyHostname("127.0.0.1"))
	require.NoError(t, leaf.VerifyHostname("localhost"))
	require.NoError(t, leaf.VerifyHostname("::1"))
	assert.Equal(t, serverCommonName, leaf.Subject.CommonName)
	assert.Contains(t, leaf.IPAddresses, net.IPv4(127, 0, 0, 1).To4())
	assert.True(t, leaf.NotAfter.After(time.Now().Add(serverValidity-24*time.Hour)))
}

// TestEnsureModesLetTheUnprivilegedReaderIn is the mode contract, and the failure it
// prevents is silent: registry-packages-proxy runs unprivileged, mounts this directory
// from the host, and reads an unreadable authority as "there is no agent on this node".
func TestEnsureModesLetTheUnprivilegedReaderIn(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki")
	material := &OnDisk{Dir: dir}
	require.NoError(t, material.Ensure())

	for _, tc := range []struct {
		path string
		mode os.FileMode
	}{
		{dir, dirMode},
		{material.Path(CAFile), certMode},
		{material.Path(CertificateFile), certMode},
		{material.Path(KeyFile), keyMode},
	} {
		info, err := os.Stat(tc.path)
		require.NoError(t, err)
		assert.Equal(t, tc.mode, info.Mode().Perm(), tc.path)
	}
}

// Material already on the node is left exactly as it is. On a bashible node the step owns
// this directory and renews it; an agent that regenerated would be a second writer of the
// one file the runtime verifies against.
func TestEnsureKeepsMaterialItDidNotWrite(t *testing.T) {
	material := &OnDisk{Dir: filepath.Join(t.TempDir(), "pki")}
	require.NoError(t, material.Ensure())

	before, err := os.ReadFile(material.Path(CAFile))
	require.NoError(t, err)

	require.NoError(t, material.Ensure())

	after, err := os.ReadFile(material.Path(CAFile))
	require.NoError(t, err)
	assert.Equal(t, before, after, "a second pass reissued the authority the runtime trusts")
}

// Half a set is not a set: a key that does not match its certificate makes every
// handshake fail, and the agent would rather replace it than serve it.
func TestEnsureReplacesUnusableMaterial(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki")
	material := &OnDisk{Dir: dir}
	require.NoError(t, material.Ensure())
	require.NoError(t, os.WriteFile(material.Path(KeyFile), []byte("not a key"), keyMode))

	require.NoError(t, material.Ensure())
	require.NoError(t, material.Ready())
}

// TestEnsureKeepsTheDirectory is the hostPath rule. The Deckhouse pod and
// registry-packages-proxy each hold a bind mount of this directory; replacing it rather
// than the files in it leaves them reading an inode nothing writes to any more.
func TestEnsureKeepsTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki")
	material := &OnDisk{Dir: dir}
	require.NoError(t, material.Ensure())

	before, err := os.Stat(dir)
	require.NoError(t, err)

	require.NoError(t, os.Remove(material.Path(CAFile)))
	require.NoError(t, material.Ensure())

	after, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after), "the directory itself was replaced")
}

// Nothing renews from the authority's key, so it is never written down.
func TestEnsureLeavesNothingElseBehind(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki")
	require.NoError(t, (&OnDisk{Dir: dir}).Ensure())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.ElementsMatch(t, []string{CAFile, CertificateFile, KeyFile}, names,
		"the authority's private key, or a temporary file, was left on the node")
}

// The certificate has to be usable as a serving certificate, which is the one thing a
// unit test of the files alone would not notice.
func TestEnsureMaterialServesATLSHandshake(t *testing.T) {
	material := &OnDisk{Dir: filepath.Join(t.TempDir(), "pki")}
	require.NoError(t, material.Ensure())

	listener, err := tls.Listen("tcp", "127.0.0.1:0", material.TLSConfig())
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.(*tls.Conn).Handshake()
			_ = conn.Close()
		}
	}()

	authority, err := material.CA()
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(authority)))

	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
	require.NoError(t, err, "the container runtime could not verify the agent")
	require.NoError(t, client.Close())
}
