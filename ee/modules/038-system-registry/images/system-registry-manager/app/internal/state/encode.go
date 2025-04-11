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

type encodeDecodeSecret interface {
	DecodeSecret(secret *corev1.Secret) error
	EncodeSecret(secret *corev1.Secret) error
}

func decodeCertKeyFromSecret(certField, keyField string, secret *corev1.Secret) (pki.CertKey, error) {
	return pki.DecodeCertKey(secret.Data[certField], secret.Data[keyField])
}

func encodeCertKeyToSecret(value pki.CertKey, certField, keyField string, secret *corev1.Secret) error {
	if secret == nil {
		return fmt.Errorf("invalid secret")
	}

	certBytes, keyBytes, err := pki.EncodeCertKey(value)
	if err != nil {
		return fmt.Errorf("cannot encode key: %w", err)
	}

	secret.Data[certField] = certBytes
	secret.Data[keyField] = keyBytes

	return nil
}

func initSecretLabels(secret *corev1.Secret) {
	// Set labels
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}

	secret.Labels[LabelModuleKey] = RegistryModuleName
	secret.Labels[LabelHeritageKey] = LabelHeritageValue
	secret.Labels[LabelManagedBy] = RegistryModuleName
}
