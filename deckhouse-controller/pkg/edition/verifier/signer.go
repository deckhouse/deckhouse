/*
Copyright 2025 Flant JSC

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

package verifier

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.mozilla.org/pkcs7"
)

// signer is used only for tests to generate modules private keys and sign modules
type signer struct {
	caCert *x509.Certificate
	caKey  *ecdsa.PrivateKey
}

func newSigner(ca *x509.Certificate, key *ecdsa.PrivateKey) *signer {
	return &signer{
		caCert: ca,
		caKey:  key,
	}
}

func (s *signer) Sign(moduleName string, modulePath string) error {
	// generate ECDSA private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	checksum, err := s.calculateChecksum(modulePath)
	if err != nil {
		return fmt.Errorf("calculate module checksum: %w", err)
	}

	subject := pkix.Name{CommonName: moduleName}

	signedCert, err := s.createSignedCert(subject, priv.PublicKey)
	if err != nil {
		return fmt.Errorf("create signed cert: %w", err)
	}

	signature, err := s.signChecksum(signedCert, priv, checksum)
	if err != nil {
		return fmt.Errorf("sign module signature: %w", err)
	}

	return os.WriteFile(filepath.Join(modulePath, "module.p7m"), signature, 0600)
}

func (s *signer) calculateChecksum(modulePath string) (string, error) {
	templatePath := filepath.Join(modulePath, "templates")
	valuesPath := filepath.Join(modulePath, "openapi")
	defPath := filepath.Join(modulePath, "module.yaml")

	return addonutils.CalculateChecksumOfPaths(templatePath, valuesPath, defPath)
}

func (s *signer) createSignedCert(subject pkix.Name, pubKey ecdsa.PublicKey) ([]byte, error) {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, s.caCert, &pubKey, s.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign certificate request: %w", err)
	}

	signedCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return signedCert, nil
}

func (s *signer) signChecksum(signedCert []byte, privKey *ecdsa.PrivateKey, checksum string) ([]byte, error) {
	sd, err := pkcs7.NewSignedData([]byte(checksum))
	if err != nil {
		return nil, fmt.Errorf("create pkcs7 signed data: %w", err)
	}

	block, _ := pem.Decode(signedCert)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse signed certificate: %w", err)
	}

	if err = sd.AddSigner(cert, privKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("add signer: %w", err)
	}

	detached, err := sd.Finish()
	if err != nil {
		return nil, fmt.Errorf("finish signing: %w", err)
	}

	return detached, nil
}
