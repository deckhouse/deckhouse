/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"

	"embeded-registry-manager/internal/utils/pki"
)

const (
	nodePKISecretType      = "system-registry/node-pki-secret"
	nodePKISecretTypeLabel = "node-pki-secret"

	nodeAuthCertCN         = "embedded-registry-auth"
	nodeDistributionCertCN = "embedded-registry-distribution"

	nodeAuthCertSecretField = "auth.crt"
	nodeAuthKeySecretField  = "auth.key"

	nodeDistributionCertSecretField = "distribution.crt"
	nodeDistributionKeySecretField  = "distribution.key"
)

var (
	NodePKISecretRegex = regexp.MustCompile(`^registry-node-(.*)-pki$`)
)

func NodePKISecretName(nodeName string) string {
	return fmt.Sprintf("registry-node-%s-pki", nodeName)
}

var _ encodeDecodeSecret = &NodePKI{}

type NodePKI struct {
	Auth         *pki.CertKey
	Distribution *pki.CertKey
}

func GenerateNodePKI(ca pki.CertKey, hosts []string) (ret NodePKI, err error) {
	var generatedPKI pki.CertKey

	generatedPKI, err = pki.GenerateCertificate(nodeAuthCertCN, hosts, ca)
	if err != nil {
		err = fmt.Errorf("cannot generate Auth PKI: %w", err)
		return
	}
	ret.Auth = &generatedPKI

	generatedPKI, err = pki.GenerateCertificate(nodeDistributionCertCN, hosts, ca)
	if err != nil {
		err = fmt.Errorf("cannot generate Distribution PKI: %w", err)
		return
	}
	ret.Distribution = &generatedPKI

	return
}

func (nc *NodePKI) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	authPKI, err := decodeCertKeyFromSecret(nodeAuthCertSecretField, nodeAuthKeySecretField, secret)
	if err != nil {
		return fmt.Errorf("cannot decode auth PKI: %w", err)
	}

	distributionPKI, err := decodeCertKeyFromSecret(nodeDistributionCertSecretField, nodeDistributionKeySecretField, secret)
	if err != nil {
		return fmt.Errorf("cannot decode distribution PKI: %w", err)
	}

	nc.Auth = &authPKI
	nc.Distribution = &distributionPKI

	return nil
}

func (nc *NodePKI) EncodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	secret.Type = nodePKISecretType

	initSecretLabels(secret)
	secret.Labels[LabelTypeKey] = nodePKISecretTypeLabel

	secret.Data = make(map[string][]byte)
	if err := encodeCertKeyToSecret(
		*nc.Auth,
		nodeAuthCertSecretField,
		nodeAuthKeySecretField,
		secret,
	); err != nil {
		return fmt.Errorf("cannot encode Auth: %w", err)
	}

	if err := encodeCertKeyToSecret(
		*nc.Distribution,
		nodeDistributionCertSecretField,
		nodeDistributionKeySecretField,
		secret,
	); err != nil {
		return fmt.Errorf("cannot encode Distribution: %w", err)
	}

	return nil
}
