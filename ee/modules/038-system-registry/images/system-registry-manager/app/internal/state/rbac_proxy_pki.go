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
	RbacProxyPKIConfigMapName        = "kube-rbac-proxy-ca.crt"
	RbacProxyPKIConfigMapCACertField = "ca.crt"
)

type RbacProxyPKI struct {
	CaCert *x509.Certificate
}

func (nc *RbacProxyPKI) DecodeConfigMap(cm *corev1.ConfigMap) error {
	if cm == nil {
		return ErrConfigMapIsNil
	}

	var err error
	caCertFieldData := []byte(cm.Data[RbacProxyPKIConfigMapCACertField])

	if nc.CaCert, err = pki.DecodeCertificate(caCertFieldData); err != nil {
		return fmt.Errorf("cannot decode certificate: %w", err)
	}
	return nil
}
