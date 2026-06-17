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
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math"
	"math/big"
	"time"

	certutil "k8s.io/client-go/util/cert"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
)

// CertConfig holds parameters for generating a single certificate.
// It embeds certutil.Config (CommonName, Organization, AltNames, Usages, NotBefore)
// and extends it with fields specific to this package.
type CertConfig struct {
	certutil.Config

	// NotAfter explicitly sets the certificate expiry time.
	// If zero, it is computed as NotBefore + CertificateValidityPeriod.
	NotAfter time.Time

	// EncryptionAlgorithm is the asymmetric key algorithm used when generating
	// a new private key for this certificate. Used by NewCertAndKey.
	EncryptionAlgorithm constants.EncryptionAlgorithmType

	// CertificateValidityPeriod is the validity duration for this certificate.
	// Ignored when NotAfter is explicitly set.
	CertificateValidityPeriod time.Duration
}

// NewPrivateKey generates a new private key of the given algorithm type.
func NewPrivateKey(keyType constants.EncryptionAlgorithmType) (crypto.Signer, error) {
	switch keyType {
	case constants.EncryptionAlgorithmECDSAP256:
		return ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	case constants.EncryptionAlgorithmECDSAP384:
		return ecdsa.GenerateKey(elliptic.P384(), cryptorand.Reader)
	}

	rsaKeySize := rsaKeySizeFromAlgorithmType(keyType)
	if rsaKeySize == 0 {
		return nil, fmt.Errorf("cannot obtain key size from unknown RSA algorithm: %q", keyType)
	}
	return rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
}

// NewSelfSignedCACert creates a new self-signed CA certificate.
//
// NotBefore is backdated by constants.CertificateBackdate (5 minutes) to tolerate
// clock skew between nodes — a certificate that starts "slightly in the past" is
// immediately valid on all nodes even if their clocks differ slightly.
//
// The self-signing is achieved by passing the same template as both template and parent
// to x509.CreateCertificate, and signing with the certificate's own key.
func NewSelfSignedCACert(cfg CertConfig, key crypto.Signer) (*x509.Certificate, error) {
	if len(cfg.CommonName) == 0 {
		return nil, fmt.Errorf("must specify a CommonName")
	}

	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, err
	}
	serial = new(big.Int).Add(serial, big.NewInt(1))

	notBefore := time.Now().UTC().Add(-constants.CertificateBackdate)

	notAfter := notBefore.Add(cfg.CertificateValidityPeriod)
	if !cfg.NotAfter.IsZero() {
		notAfter = cfg.NotAfter
	}

	RemoveDuplicateAltNames(&cfg.AltNames)

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:              cfg.AltNames.DNSNames,
		IPAddresses:           cfg.AltNames.IPs,
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, &certTmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(certDERBytes)
}

// NewSignedCert creates a leaf certificate signed by the given CA.
func NewSignedCert(cfg CertConfig, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, error) {
	if len(cfg.CommonName) == 0 {
		return nil, fmt.Errorf("must specify a CommonName")
	}

	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, err
	}
	serial = new(big.Int).Add(serial, big.NewInt(1))

	RemoveDuplicateAltNames(&cfg.AltNames)

	now := time.Now().UTC().Add(-constants.CertificateBackdate)
	if now.Before(caCert.NotBefore) {
		return nil, fmt.Errorf("cert cannot be newer than ca certificate")
	}

	notBefore := now
	if !cfg.NotBefore.IsZero() {
		notBefore = cfg.NotBefore
	}

	notAfter := notBefore.Add(cfg.CertificateValidityPeriod)
	if !cfg.NotAfter.IsZero() {
		notAfter = cfg.NotAfter
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:              cfg.AltNames.DNSNames,
		IPAddresses:           cfg.AltNames.IPs,
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           cfg.Usages,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(certDERBytes)
}

// NewCertAndKey generates a new private key and a leaf certificate signed by the given CA.
// It is a convenience wrapper around NewPrivateKey and NewSignedCert.
func NewCertAndKey(caCert *x509.Certificate, caKey crypto.Signer, cfg CertConfig) (*x509.Certificate, crypto.Signer, error) {
	if len(cfg.Usages) == 0 {
		return nil, nil, fmt.Errorf("must specify at least one ExtKeyUsage")
	}

	key, err := NewPrivateKey(cfg.EncryptionAlgorithm)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create private key: %w", err)
	}

	cert, err := NewSignedCert(cfg, key, caCert, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to sign certificate: %w", err)
	}

	return cert, key, nil
}

// rsaKeySizeFromAlgorithmType returns the key size in bits for a known RSA algorithm.
// Returns 0 for unknown types. Returns the default size of 2048 for an empty type.
func rsaKeySizeFromAlgorithmType(keyType constants.EncryptionAlgorithmType) int {
	switch keyType {
	case constants.EncryptionAlgorithmRSA2048, "":
		return 2048
	case constants.EncryptionAlgorithmRSA3072:
		return 3072
	case constants.EncryptionAlgorithmRSA4096:
		return 4096
	default:
		return 0
	}
}
