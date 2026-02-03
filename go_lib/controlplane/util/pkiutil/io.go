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

package pkiutil

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"path/filepath"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

// LoadKey loads a private key from the given file path.
// Only RSA and ECDSA formats are accepted.
func LoadKey(path string) (crypto.Signer, error) {
	return loadKey(path)
}

// LoadCertAndKey loads an X.509 certificate and its corresponding private key from pkiDir.
// The files are expected at <pkiDir>/<name>.crt and <pkiDir>/<name>.key.
func LoadCertAndKey(pkiDir, name string) (*x509.Certificate, crypto.Signer, error) {
	cert, err := loadCert(filepath.Join(pkiDir, name+".crt"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	key, err := loadKey(filepath.Join(pkiDir, name+".key"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load key: %w", err)
	}

	return cert, key, nil
}

func loadCert(path string) (*x509.Certificate, error) {
	certs, err := certutil.CertsFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't load certificate file %s: %w", path, err)
	}
	// Safely pick the first one because the sender's certificate must come first in the list.
	// For details, see: https://www.rfc-editor.org/rfc/rfc4346#section-7.4.2
	return certs[0], nil
}

func loadKey(path string) (crypto.Signer, error) {
	privKey, err := keyutil.PrivateKeyFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't load private key file %s: %w", path, err)
	}

	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		return k, nil
	case *ecdsa.PrivateKey:
		return k, nil
	default:
		return nil, fmt.Errorf("private key file %s is neither in RSA nor ECDSA format", path)
	}
}
