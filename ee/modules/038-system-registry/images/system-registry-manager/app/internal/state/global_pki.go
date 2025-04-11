/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

const (
	PKISecretName = "registry-pki"

	caSecretType      = "registry/pki"
	caSecretTypeLabel = "registry-pki"

	caCertSecretField = "ca.crt"
	caKeySecretField  = "ca.key"

	tokenCertSecretField = "token.crt"
	tokenKeySecretField  = "token.key"
)

type GlobalPKI struct {
	CA    *pki.CertKey
	Token *pki.CertKey
}

func (gp *GlobalPKI) Validate() error {
	err := pki.ValidateCertWithCAChain(gp.Token.Cert, gp.CA.Cert)
	if err != nil {
		return fmt.Errorf("error validating Token certificate: %w", err)
	}

	return nil
}

func (gp *GlobalPKI) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	caPKI, err := decodeCertKeyFromSecret(caCertSecretField, caKeySecretField, secret)

	if err != nil {
		return fmt.Errorf("cannot decode CA PKI: %w", err)
	}

	tokenPKI, err := decodeCertKeyFromSecret(tokenCertSecretField, tokenKeySecretField, secret)

	if err != nil {
		return fmt.Errorf("cannot decode Token PKI: %w", err)
	}

	gp.CA = &caPKI
	gp.Token = &tokenPKI

	return nil
}
