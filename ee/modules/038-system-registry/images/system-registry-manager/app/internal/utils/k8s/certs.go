/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s

import (
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
)

const (
	RegistryCACert   = "registry-ca.crt"
	RegistryCAKey    = "registry-ca.key"
	AuthCert         = "auth.crt"
	AuthKey          = "auth.key"
	AuthTokenCert    = "token.crt"
	AuthTokenKey     = "token.key"
	DistributionCert = "distribution.crt"
	DistributionKey  = "distribution.key"
)

type Certificate struct {
	Cert []byte
	Key  []byte
}

// set cfssl global log level to fatal
func init() {
	cfssllog.Level = cfssllog.LevelFatal
}

// generateCA generates a new CA certificate and key.
func generateCA() (caCertPEM []byte, caKeyPEM []byte, err error) {

	caRequest := &csr.CertificateRequest{
		CN: "embedded-registry-ca",
		CA: &csr.CAConfig{
			Expiry: "87600h", // 10 years
		},
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
	}

	caCertPEM, _, caKeyPEM, err = initca.New(caRequest)
	if err != nil {
		return nil, nil, err
	}

	return caCertPEM, caKeyPEM, nil
}

func Validator(req *csr.CertificateRequest) error {
	return nil
}

// generateCertificate generates a new certificate and key signed by the provided CA certificate and key.
func generateCertificate(commonName string, hosts []string, caCertPEM []byte, caKeyPEM []byte) (certPEM, keyPEM []byte, err error) {

	req := csr.CertificateRequest{
		CN: commonName,
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
		Hosts: hosts,
	}

	// generate a CSR and private key
	g := &csr.Generator{Validator: Validator}
	csrPEM, keyPEM, err := g.ProcessRequest(&req)
	if err != nil {
		return nil, nil, err
	}

	// parse CA certificate and key
	caCert, err := helpers.ParseCertificatePEM(caCertPEM)
	if err != nil {
		return nil, nil, err
	}

	caKey, err := helpers.ParsePrivateKeyPEM(caKeyPEM)
	if err != nil {
		return nil, nil, err
	}

	// create a signer
	s, err := local.NewSigner(caKey, caCert, signer.DefaultSigAlgo(caKey), nil)
	if err != nil {
		return nil, nil, err
	}

	// create a sign request
	signReq := signer.SignRequest{
		Request:  string(csrPEM),
		NotAfter: caCert.NotAfter.Add(-1 * time.Hour),
	}

	// sign the certificate
	certPEM, err = s.Sign(signReq)
	if err != nil {
		return nil, nil, err
	}

	return certPEM, keyPEM, nil
}
