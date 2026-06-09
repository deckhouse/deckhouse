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
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"k8s.io/client-go/util/keyutil"
)

const certificateBlockType = "CERTIFICATE"

// EncodeCertificate returns the PEM-encoded representation of cert.
func EncodeCertificate(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  certificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func ParseCertificatePEM(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("block not found")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, nil
}

func ParsePrivateKeyPEM(pemData []byte) (crypto.Signer, error) {
	key, err := keyutil.ParsePrivateKeyPEM(pemData)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return key.(crypto.Signer), nil
}

func MarshalPrivateKeyToPEM(privateKey crypto.Signer) ([]byte, error) {
	return keyutil.MarshalPrivateKeyToPEM(privateKey)
}
