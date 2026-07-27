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
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

// readCertAndKey loads a certificate and its corresponding private key from pkiDir.
// Both files must be present; if either is missing, the error wraps os.ErrNotExist.
func readCertAndKey(pkiDir, name string) (*x509.Certificate, crypto.Signer, error) {
	return pkiutil.LoadCertAndKey(pkiDir, name)
}

func certPath(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.crt", name))
}

func keyPath(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.key", name))
}

// isNotExistError reports whether err (or any error in its chain) indicates
// that a file or directory does not exist.
func isNotExistError(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// writeCertAndKey writes the private key first, then the certificate.
// The key is written first so that if the process crashes between the two writes,
// the certificate file (which is used as the existence check) is absent,
// and the next run will regenerate both files cleanly.
func writeCertAndKey(pkiDir, name string, cert *x509.Certificate, key crypto.Signer) error {
	if err := writeKey(pkiDir, name, key); err != nil {
		return fmt.Errorf("couldn't write key: %w", err)
	}
	return writeCert(pkiDir, name, cert)
}

func writeCert(pkiDir, name string, cert *x509.Certificate) error {
	certificatePath := certPath(pkiDir, name)

	if err := os.MkdirAll(filepath.Dir(certificatePath), 0o700); err != nil {
		return fmt.Errorf("couldn't create directory %q: %w", filepath.Dir(certificatePath), err)
	}

	return pkiutil.WriteFileAtomically(certificatePath, pkiutil.EncodeCertificate(cert), 0o600)
}

func writeKey(pkiDir, name string, key crypto.Signer) error {
	privateKeyPath := keyPath(pkiDir, name)

	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0o700); err != nil {
		return fmt.Errorf("couldn't create directory %q: %w", filepath.Dir(privateKeyPath), err)
	}

	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return fmt.Errorf("unable to marshal private key to PEM: %w", err)
	}

	return pkiutil.WriteFileAtomically(privateKeyPath, encoded, 0o600)
}

// writeSAPublicKey encodes the public part of key in PKIX PEM format and writes it to sa.pub.
// It uses a separate code path from writeKey because the public key format (PKIX "PUBLIC KEY")
// differs from the private key format handled by keyutil.MarshalPrivateKeyToPEM.
func writeSAPublicKey(pkiDir string, key crypto.Signer) error {
	pubKeyDER, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return fmt.Errorf("unable to marshal SA public key to DER: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyDER,
	})

	if err := pkiutil.WriteFileAtomically(filepath.Join(pkiDir, "sa.pub"), publicKeyPEM, 0o600); err != nil {
		return fmt.Errorf("unable to write SA public key: %w", err)
	}

	return nil
}
