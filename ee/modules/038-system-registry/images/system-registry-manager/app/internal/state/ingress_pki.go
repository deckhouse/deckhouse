/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"crypto/x509"
	"fmt"
	corev1 "k8s.io/api/core/v1"

	"embeded-registry-manager/internal/utils/pki"
)

const (
	IngressPKIConfigMapName              = "kube-rbac-proxy-ca.crt"
	IngressPKIConfigMapClientCACertField = "ca.crt"
)

type IngressPKI struct {
	ClientCACert *x509.Certificate
}

func (nc *IngressPKI) DecodeConfigMap(cm *corev1.ConfigMap) error {
	if cm == nil {
		return ErrConfigMapIsNil
	}

	var err error
	fieldData, ok := cm.Data[IngressPKIConfigMapClientCACertField]

	if !ok {
		return fmt.Errorf("empty field %v", IngressPKIConfigMapClientCACertField)
	}

	if nc.ClientCACert, err = pki.DecodeCertificate([]byte(fieldData)); err != nil {
		return fmt.Errorf("cannot decode certificate: %w", err)
	}
	return nil
}
