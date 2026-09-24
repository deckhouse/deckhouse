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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// What the material is, matching what the bashible step generates with cfssl
// (candi/bashible/common-steps/all/053_configure_registry_agent.sh.tpl). The two have to
// agree: one node kind gets its material from there and the other from here, and an
// operator comparing them should find the same thing.
const (
	caCommonName     = "registry-agent-ca"
	serverCommonName = "registry-agent"

	caValidity     = 175200 * time.Hour // 20 years
	serverValidity = 87600 * time.Hour  // 10 years

	keyBits = 2048
)

// The modes are a contract, not a preference.
//
// Two things on the node verify the agent and only one of them is root: the container
// runtime, which is, and registry-packages-proxy, which is not — it runs unprivileged in
// a pod that mounts this directory from the host. An unreadable authority reads to it as
// "there is no agent here", and it quietly fetches from somewhere else instead.
const (
	dirMode  os.FileMode = 0o755
	certMode os.FileMode = 0o644
	keyMode  os.FileMode = 0o600
)

// Ensure makes the directory hold usable material, generating a fresh authority and
// serving certificate when it does not.
//
// This is what lets an Engine node run the agent at all. There, the material has no other
// writer: /etc/kubernetes is a tmpfs, and the object that puts the agent on such a node
// carries a pod manifest and nothing else, so there is no step to generate anything ahead
// of it. The certificate is self-signed and node-local — the only party that ever has to
// trust it is the runtime on this very node — so generating it here costs nothing that
// generating it in a shell script did not.
//
// Deliberately only when the material is missing or unusable, never merely because it is
// close to expiring. On a bashible node the step is the writer and renews at thirty days;
// an agent that also renewed would be a second writer of the one file the runtime verifies
// against, racing it for no gain. On an Engine node the question does not arise: the
// directory is empty on every boot, so the material is never old.
func (o *OnDisk) Ensure() error {
	if err := os.MkdirAll(o.dir(), dirMode); err != nil {
		return fmt.Errorf("creating the agent certificate directory: %w", err)
	}
	// Set explicitly: MkdirAll applies the process umask, and a directory the unprivileged
	// reader above cannot enter is the failure this mode exists to prevent.
	if err := os.Chmod(o.dir(), dirMode); err != nil {
		return fmt.Errorf("setting the mode of the agent certificate directory: %w", err)
	}

	if o.Ready() == nil {
		return nil
	}

	material, err := generate()
	if err != nil {
		return err
	}

	// Written in place, one file at a time, and never by replacing the directory: two
	// pods hold a bind mount of this very directory, and a replaced one leaves them
	// looking at an inode nothing writes to any more.
	//
	// The order is the bashible step's, and so is the window it leaves: between the first
	// rename and the last, the authority and the certificate are from different
	// generations and a handshake fails. It closes in microseconds, and both readers of
	// either file read it again on the next attempt rather than caching it.
	for _, file := range []struct {
		name    string
		content []byte
		mode    os.FileMode
	}{
		{CAFile, material.caPEM, certMode},
		{CertificateFile, material.certificatePEM, certMode},
		{KeyFile, material.keyPEM, keyMode},
	} {
		if err := writeInPlace(o.Path(file.name), file.content, file.mode); err != nil {
			return err
		}
	}

	return nil
}

// generated is one complete set. The authority's private key is not among them: nothing
// renews from it — a renewal generates a fresh authority and the agent rewrites the
// runtime's copy of it — so keeping it would be a private key with no reader.
type generated struct {
	caPEM          []byte
	certificatePEM []byte
	keyPEM         []byte
}

func generate() (*generated, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, fmt.Errorf("generating the agent authority key: %w", err)
	}

	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: caCommonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating the agent authority: %w", err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("parsing the agent authority back: %w", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, fmt.Errorf("generating the agent key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("choosing a serial number: %w", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: serverCommonName},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(serverValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Loopback names only. The agent serves the runtime on this node and nothing
		// else, so naming the node's own address would add a reason to reissue — the
		// address changing — without adding anything that uses it.
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	serverDER, err := x509.CreateCertificate(
		rand.Reader, serverTemplate, caCertificate, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating the agent certificate: %w", err)
	}

	return &generated{
		caPEM:          pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		certificatePEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}),
		keyPEM: pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey),
		}),
	}, nil
}

// writeInPlace replaces one file atomically, leaving the directory it is in alone.
//
// Through a temporary file in the same directory, because a rename is only atomic within
// one filesystem and because a reader must never see a half-written certificate: the
// runtime reads these while pulls are in flight.
func writeInPlace(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("creating a temporary file next to %s: %w", path, err)
	}
	// Removed unless the rename below took it; a failure part-way must not leave a
	// readable private key lying about under a name nothing cleans up.
	defer func() { _ = os.Remove(temporary.Name()) }()

	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	// Before the rename, so the file is never visible under its real name with the
	// default mode CreateTemp gives it.
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("setting the mode of %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("putting %s in place: %w", path, err)
	}
	return nil
}
