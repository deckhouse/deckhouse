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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issue writes a self-signed certificate, the way the bashible step does at bootstrap.
func issue(t *testing.T, dir, commonName string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, CertificateFile),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, KeyFile),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, CAFile),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644))
}

func TestReady(t *testing.T) {
	dir := t.TempDir()
	material := &OnDisk{Dir: dir}

	// A node where the bashible step has not run yet is an ordinary state, and saying
	// what is missing beats failing at the first handshake.
	require.Error(t, material.Ready())

	issue(t, dir, "registry-agent")
	require.NoError(t, material.Ready())

	authority, err := material.CA()
	require.NoError(t, err)
	assert.Contains(t, authority, "BEGIN CERTIFICATE")
}

// TestCertificateFollowsRotation is why the material is read from disk rather than held
// in memory: restarting the agent to pick up a new certificate would mean a window in
// which nothing on the node can pull.
func TestCertificateFollowsRotation(t *testing.T) {
	dir := t.TempDir()
	material := &OnDisk{Dir: dir}
	issue(t, dir, "registry-agent")

	before, err := material.Certificate(nil)
	require.NoError(t, err)

	issue(t, dir, "registry-agent-rotated")

	after, err := material.Certificate(nil)
	require.NoError(t, err)
	assert.NotEqual(t, before.Certificate[0], after.Certificate[0])
}

// TestCertificateParsesOnlyOnChange matters because this runs on every handshake and a
// pull is many connections.
func TestCertificateParsesOnlyOnChange(t *testing.T) {
	dir := t.TempDir()
	material := &OnDisk{Dir: dir}
	issue(t, dir, "registry-agent")

	first, err := material.Certificate(nil)
	require.NoError(t, err)
	second, err := material.Certificate(nil)
	require.NoError(t, err)

	assert.Same(t, first, second)
}

func TestCertificateRejectsAMismatchedPair(t *testing.T) {
	dir := t.TempDir()
	issue(t, dir, "registry-agent")

	// A key from a different certificate, which is what a half-finished rotation looks
	// like on disk.
	other := t.TempDir()
	issue(t, other, "somebody-else")
	content, err := os.ReadFile(filepath.Join(other, KeyFile))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, KeyFile), content, 0o600))

	_, err = (&OnDisk{Dir: dir}).Certificate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "do not form a pair")
}

func TestCARejectsAnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	issue(t, dir, "registry-agent")
	require.NoError(t, os.WriteFile(filepath.Join(dir, CAFile), nil, 0o644))

	// An empty authority would be written into the runtime's drop-in and make every
	// pull on the node fail verification, so it has to be caught here.
	_, err := (&OnDisk{Dir: dir}).CA()
	assert.Error(t, err)
}

func TestTLSConfigReloads(t *testing.T) {
	dir := t.TempDir()
	issue(t, dir, "registry-agent")

	config := (&OnDisk{Dir: dir}).TLSConfig()
	require.NotNil(t, config.GetCertificate)

	certificate, err := config.GetCertificate(nil)
	require.NoError(t, err)
	assert.NotNil(t, certificate)
}

func TestDefaultDir(t *testing.T) {
	assert.Equal(t, "/etc/kubernetes/registry-agent/pki", (&OnDisk{}).dir())
	assert.Equal(t, "/etc/kubernetes/registry-agent/pki/ca.crt", (&OnDisk{}).Path(CAFile))
}
