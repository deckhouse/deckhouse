/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

const (
	AuthCertCN         = "registry-auth"
	DistributionCertCN = "registry-distribution"
)

func ProcessDistributionCertPair(log go_hook.Logger, certPair CertPair, ca pki.CertKey) (CertPair, error) {
	hosts := []string{"127.0.0.1", "localhost", helpers.RegistryServiceDNSName}
	return ProcessCertPair(log, certPair, ca, hosts, DistributionCertCN)
}

func ProcessAuthCertPair(log go_hook.Logger, certPair CertPair, ca pki.CertKey) (CertPair, error) {
	hosts := []string{"127.0.0.1", "localhost", helpers.RegistryServiceDNSName}
	return ProcessCertPair(log, certPair, ca, hosts, AuthCertCN)
}

func ProcessCertPair(log go_hook.Logger, certPair CertPair, ca pki.CertKey, hosts []string, cn string) (CertPair, error) {
	isValid, err := certPair.IsValid(ca, hosts, cn)
	if !isValid {
		log.Warn("Certificate pair is invalid, generating a new one.", "cn", cn, "error", err)
		if err = certPair.Generate(ca, hosts, cn); err != nil {
			return certPair, fmt.Errorf("failed to generate certificate pair: %w", err)
		}
	}
	return certPair, nil
}

type CertPair struct {
	Cert string
	Key  string
}

func (certPair *CertPair) IsValid(ca pki.CertKey, hosts []string, expectedCN string) (bool, error) {
	if certPair.Cert == "" || certPair.Key == "" {
		return false, fmt.Errorf("certificate or key is empty")
	}

	decodedCertKey, err := pki.DecodeCertKey([]byte(certPair.Cert), []byte(certPair.Key))
	if err != nil {
		return false, fmt.Errorf("failed to decode the certificate pair: %w", err)
	}

	if err := pki.ValidateCertWithCAChain(decodedCertKey.Cert, ca.Cert); err != nil {
		return false, fmt.Errorf("failed certificate validation: %w", err)
	}

	for _, host := range hosts {
		if err := decodedCertKey.Cert.VerifyHostname(host); err != nil {
			return false, fmt.Errorf("hostname verification failed for \"%v\": %w", host, err)
		}
	}
	return true, nil
}

func (certPair *CertPair) Generate(ca pki.CertKey, hosts []string, cn string) error {
	newCertPair, err := pki.GenerateCertificate(cn, ca, hosts...)
	if err != nil {
		return fmt.Errorf("failed to generate certificate pair: %w", err)
	}

	certPair.Cert = string(pki.EncodeCertificate(newCertPair.Cert))

	encodedKey, err := pki.EncodePrivateKey(newCertPair.Key)
	if err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}
	certPair.Key = string(encodedKey)
	return nil
}
