/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/cloudflare/cfssl/helpers"
)

// EncodePrivateKey encodes crypto.Signer private key to PEM fomat
func EncodePrivateKey(signer crypto.Signer) ([]byte, error) {
	var derBytes []byte
	var err error

	switch key := signer.(type) {
	case *ecdsa.PrivateKey:
		// Marshal ECDSA private key
		derBytes, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ECDSA private key: %v", err)
		}
	case *rsa.PrivateKey:
		// Marshal RSA private key
		derBytes = x509.MarshalPKCS1PrivateKey(key)
	default:
		return nil, fmt.Errorf("unsupported key type %T", signer)
	}

	// Encode to PEM format
	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derBytes,
	}

	return pem.EncodeToMemory(pemBlock), nil
}

// DecodePrivateKey decodes private key from PEM format to crypto.Signer
func DecodePrivateKey(pemData []byte) (crypto.Signer, error) {
	return helpers.ParsePrivateKeyPEM(pemData)
}

// EncodeCertificate encodes an X.509 certificate to PEM format.
func EncodeCertificate(cert *x509.Certificate) []byte {
	return helpers.EncodeCertificatePEM(cert)
}

// DecodeCertificate decodes a PEM-encoded X.509 certificate into an *x509.Certificate.
func DecodeCertificate(pemData []byte) (*x509.Certificate, error) {
	return helpers.ParseCertificatePEM(pemData)
}

// ComparePublicKeys compares two public keys
// Keys must implement
//
//	interface{
//	    Public() crypto.PublicKey
//	    Equal(x crypto.PrivateKey) bool
//	}
//
// which already implemented in standart library for all supported key types
func ComparePublicKeys(pubKey1, pubKey2 crypto.PublicKey) (bool, error) {
	type IsEqual interface {
		Equal(x crypto.PublicKey) bool
	}

	if k1, ok := pubKey1.(IsEqual); ok {
		return k1.Equal(pubKey2), nil
	} else if k2, ok := pubKey2.(IsEqual); ok {
		return k2.Equal(pubKey1), nil
	}

	return false, errors.New("equality comparsion is for keys is not supported")
}

// ValidateCertWithCAChain validates a server certificate using a chain of CA certificates.
// The function expects at least one CA certificate and can handle multiple intermediate CA certificates
// along with the root CA. The last certificate in the list is treated as the root CA.
func ValidateCertWithCAChain(serverCert *x509.Certificate, caCerts ...*x509.Certificate) error {
	// Ensure that there is at least one CA certificate provided
	if len(caCerts) == 0 {
		return fmt.Errorf("at least one CA certificate must be provided")
	}

	// Create a CertPool for the root CAs (the last cert in the list is treated as the root CA)
	rootCertPool := x509.NewCertPool()

	// Add the last certificate in the chain as the root CA to the CertPool
	rootCertPool.AddCert(caCerts[len(caCerts)-1])

	// Create an intermediate CertPool to hold other CA certificates (all except the last one)
	intermediateCertPool := x509.NewCertPool()

	// Add all the intermediate certificates to the intermediateCertPool
	for _, cert := range caCerts[:len(caCerts)-1] {
		intermediateCertPool.AddCert(cert)
	}

	// Verify the server certificate using the root and intermediate CA pools
	_, err := serverCert.Verify(x509.VerifyOptions{
		Roots:         rootCertPool,         // The root CA pool
		Intermediates: intermediateCertPool, // The intermediate CAs pool
	})
	if err != nil {
		// Return an error if certificate verification fails
		return fmt.Errorf("certificate verification failed: %v", err)
	}

	// Return nil if the certificate is successfully verified
	return nil
}
