/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
)

type CertKey struct {
	Cert *x509.Certificate
	Key  crypto.Signer
}

// GenerateCA generates a new CA certificate and key.
func GenerateCACertificate(commonName string) (CertKey, error) {
	var ret CertKey

	req := &csr.CertificateRequest{
		CN: commonName,
		CA: &csr.CAConfig{
			Expiry: "87600h", // 10 years
		},
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
	}

	cert, _, key, err := initca.New(req)
	if err != nil {
		return ret, fmt.Errorf("cannot initlize CA: %w", err)
	}

	ret.Cert, err = DecodeCertificate(cert)
	if err != nil {
		return ret, fmt.Errorf("cannot decode CA cert: %w", err)
	}

	ret.Key, err = DecodePrivateKey(key)
	if err != nil {
		return ret, fmt.Errorf("cannot decode CA key: %w", err)
	}

	return ret, nil
}

// GenerateCertificate generates a new certificate and key signed by the provided CA certificate and key.
func GenerateCertificate(commonName string, hosts []string, ca CertKey) (CertKey, error) {
	var ret CertKey

	req := csr.CertificateRequest{
		CN: commonName,
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
		Hosts: hosts,
	}

	// generate a CSR and private key
	g := &csr.Generator{
		Validator: func(cr *csr.CertificateRequest) error {
			return nil
		},
	}

	csr, key, err := g.ProcessRequest(&req)
	if err != nil {
		return ret, fmt.Errorf("cannot generate CSR: %w", err)
	}

	// create a signer
	s, err := local.NewSigner(ca.Key, ca.Cert, signer.DefaultSigAlgo(ca.Key), nil)
	if err != nil {
		return ret, fmt.Errorf("cannot create signer: ")
	}

	// create a sign request
	signReq := signer.SignRequest{
		Request:  string(csr),
		NotAfter: ca.Cert.NotAfter.Add(-1 * time.Hour),
	}

	// sign the certificate
	cert, err := s.Sign(signReq)
	if err != nil {
		return ret, fmt.Errorf("cannot sign: %w", err)
	}

	ret.Cert, err = DecodeCertificate(cert)
	if err != nil {
		return ret, fmt.Errorf("cannot decode CA cert: %w", err)
	}

	ret.Key, err = DecodePrivateKey(key)
	if err != nil {
		return ret, fmt.Errorf("cannot decode CA key: %w", err)
	}

	return ret, nil
}
