/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

const (
	nodeAuthCertCN         = "embedded-registry-auth"
	nodeDistributionCertCN = "embedded-registry-distribution"
)

func NodePKISecretName(nodeName string) string {
	return fmt.Sprintf("registry-node-%s-pki", nodeName)
}

type NodePKI struct {
	Auth         *pki.CertKey
	Distribution *pki.CertKey
}

func GenerateNodePKI(ca pki.CertKey, hosts []string) (NodePKI, error) {
	var (
		ret          NodePKI
		err          error
		generatedPKI pki.CertKey
	)

	generatedPKI, err = pki.GenerateCertificate(nodeAuthCertCN, ca, hosts...)
	if err != nil {
		err = fmt.Errorf("cannot generate Auth PKI: %w", err)
		return ret, err
	}
	ret.Auth = &generatedPKI

	generatedPKI, err = pki.GenerateCertificate(nodeDistributionCertCN, ca, hosts...)
	if err != nil {
		err = fmt.Errorf("cannot generate Distribution PKI: %w", err)
		return ret, err
	}
	ret.Distribution = &generatedPKI

	return ret, err
}

func (nc *NodePKI) DecodeServicesConfig(config NodeServicesConfig) error {
	pkiConfig := config.Config

	authPKI, err := pki.DecodeCertKey([]byte(pkiConfig.AuthCert), []byte(pkiConfig.AuthKey))
	if err != nil {
		return fmt.Errorf("cannot decode auth PKI: %w", err)
	}

	distributionPKI, err := pki.DecodeCertKey([]byte(pkiConfig.DistributionCert), []byte(pkiConfig.DistributionKey))
	if err != nil {
		return fmt.Errorf("cannot decode distribution PKI: %w", err)
	}

	nc.Auth = &authPKI
	nc.Distribution = &distributionPKI

	return nil
}
