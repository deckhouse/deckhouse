/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"embeded-registry-manager/internal/utils/pki"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func DecodeCertKeyFromSecret(certField, keyField string, secret *corev1.Secret) (ret pki.CertKey, err error) {
	if ret.Cert, err = pki.DecodeCertificate(secret.Data[certField]); err != nil {
		err = fmt.Errorf("cannot decode certificate: %w", err)
		return
	}

	if ret.Key, err = pki.DecodePrivateKey(secret.Data[keyField]); err != nil {
		err = fmt.Errorf("cannot decode key: %w", err)
		return
	}

	var equal bool
	if equal, err = pki.ComparePublicKeys(ret.Cert.PublicKey, ret.Key.Public()); err != nil {
		err = fmt.Errorf("cannot match CA certificate and key: %w", err)
	} else if !equal {
		err = fmt.Errorf("certificate and key does not match")
	}

	return
}

func EncodeCertKeyToSecret(value pki.CertKey, certField, keyField string, secret *corev1.Secret) error {
	if secret == nil {
		return fmt.Errorf("invalid secret")
	}

	certBytes := pki.EncodeCertificate(value.Cert)

	keyBytes, err := pki.EncodePrivateKey(value.Key)
	if err != nil {
		return fmt.Errorf("cannot encode key: %w", err)
	}

	secret.Data[certField] = certBytes
	secret.Data[keyField] = keyBytes

	return nil
}
