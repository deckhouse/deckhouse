/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inclusterproxy

import (
	"crypto/x509"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	inclusterProxyAuthCertCN         = "registry-auth"
	inclusterProxyDistributionCertCN = "registry-distribution"
)

type inclusterProxyPKI struct {
	Auth         *pki.CertKey
	Distribution *pki.CertKey
}

func (nc *inclusterProxyPKI) generate(ca pki.CertKey, hosts []string) error {
	authPKI, err := pki.GenerateCertificate(inclusterProxyAuthCertCN, ca, hosts...)
	if err != nil {
		return fmt.Errorf("cannot generate Auth PKI: %w", err)
	}

	distributionPKI, err := pki.GenerateCertificate(inclusterProxyDistributionCertCN, ca, hosts...)
	if err != nil {
		return fmt.Errorf("cannot generate Distribution PKI: %w", err)
	}

	nc.Auth = &authPKI
	nc.Distribution = &distributionPKI
	return nil
}

func (nc *inclusterProxyPKI) loadFromConfig(config InclusterProxyConfig, ca *x509.Certificate, hosts []string) error {
	if config.AuthCert == "" ||
		config.AuthKey == "" ||
		config.DistributionCert == "" ||
		config.DistributionKey == "" {
		return fmt.Errorf("missing PKI configuration")
	}

	authPKI, err := pki.DecodeCertKey([]byte(config.AuthCert), []byte(config.AuthKey))
	if err != nil {
		return fmt.Errorf("cannot decode Auth PKI: %w", err)
	}

	distributionPKI, err := pki.DecodeCertKey([]byte(config.DistributionCert), []byte(config.DistributionKey))
	if err != nil {
		return fmt.Errorf("cannot decode Distribution PKI: %w", err)
	}

	if err := pki.ValidateCertWithCAChain(authPKI.Cert, ca); err != nil {
		return fmt.Errorf("error validating Auth certificate: %w", err)
	}

	if err := pki.ValidateCertWithCAChain(distributionPKI.Cert, ca); err != nil {
		return fmt.Errorf("error validating Distribution certificate: %w", err)
	}

	for _, host := range hosts {
		if err := authPKI.Cert.VerifyHostname(host); err != nil {
			return fmt.Errorf("hostname \"%v\" not supported by Auth certificate: %w", host, err)
		}
		if err := distributionPKI.Cert.VerifyHostname(host); err != nil {
			return fmt.Errorf("hostname \"%v\" not supported by Distribution certificate: %w", host, err)
		}
	}

	nc.Auth = &authPKI
	nc.Distribution = &distributionPKI
	return nil
}

func (nc *inclusterProxyPKI) Process(log go_hook.Logger, ca pki.CertKey, config InclusterProxyConfig) error {
	certHosts := []string{"127.0.0.1", "localhost", helpers.RegistryServiceDNSName}
	log = log.With("action", "ProcessInclusterProxyPKI")

	err := nc.loadFromConfig(config, ca.Cert, certHosts)
	if err == nil {
		return nil
	}
	log.Warn("Failed to decode PKI from config", "error", err)

	log.Info("Generating new PKI")
	if err := nc.generate(ca, certHosts); err != nil {
		return fmt.Errorf("cannot generate new PKI: %w", err)
	}
	return nil
}
