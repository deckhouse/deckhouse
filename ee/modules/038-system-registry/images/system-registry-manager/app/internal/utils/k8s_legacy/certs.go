/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s_legacy

import (
	"embeded-registry-manager/internal/utils/pki"
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

	CACommonName = "embedded-registry-ca"
)

type Certificate struct {
	Cert []byte
	Key  []byte
}

// generateCA generates a new CA certificate and key.
func generateCA() (caCertPEM []byte, caKeyPEM []byte, err error) {
	var caPKI pki.CertKey

	if caPKI, err = pki.GenerateCACertificate(CACommonName); err != nil {
		return
	}

	caCertPEM = pki.EncodeCertificate(caPKI.Cert)
	caKeyPEM, err = pki.EncodePrivateKey(caPKI.Key)

	return
}

// generateCertificate generates a new certificate and key signed by the provided CA certificate and key.
func generateCertificate(commonName string, hosts []string, caCertPEM []byte, caKeyPEM []byte) (certPEM, keyPEM []byte, err error) {
	var caPKI, retPKI pki.CertKey

	if caPKI.Cert, err = pki.DecodeCertificate(caCertPEM); err != nil {
		return
	}

	if caPKI.Key, err = pki.DecodePrivateKey(caKeyPEM); err != nil {
		return
	}

	if retPKI, err = pki.GenerateCertificate(commonName, hosts, caPKI); err != nil {
		return
	}

	certPEM = pki.EncodeCertificate(retPKI.Cert)
	keyPEM, err = pki.EncodePrivateKey(retPKI.Key)

	return
}
