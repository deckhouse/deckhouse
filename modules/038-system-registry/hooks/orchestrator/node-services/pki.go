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

package nodeservices

import (
	"crypto/x509"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	nodeservices "github.com/deckhouse/deckhouse/go_lib/registry/models/node-services"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-system-registry/hooks/helpers"
)

const (
	nodeAuthCertCN         = "registry-auth"
	nodeDistributionCertCN = "registry-distribution"
)

type nodePKI struct {
	Auth         *pki.CertKey
	Distribution *pki.CertKey
}

func (nc *nodePKI) generate(ca pki.CertKey, hosts []string) error {
	authPKI, err := pki.GenerateCertificate(nodeAuthCertCN, ca, hosts...)
	if err != nil {
		return fmt.Errorf("cannot generate Auth PKI: %w", err)
	}

	distributionPKI, err := pki.GenerateCertificate(nodeDistributionCertCN, ca, hosts...)
	if err != nil {
		return fmt.Errorf("cannot generate Distribution PKI: %w", err)
	}

	nc.Auth = &authPKI
	nc.Distribution = &distributionPKI

	return nil
}

func (nc *nodePKI) loadFromConfig(config nodeservices.Config, ca *x509.Certificate, hosts []string) error {
	authPKI, err := pki.DecodeCertKey(
		[]byte(config.AuthCert), []byte(config.AuthKey),
	)
	if err != nil {
		return fmt.Errorf("cannot decode auth PKI: %w", err)
	}

	distributionPKI, err := pki.DecodeCertKey(
		[]byte(config.DistributionCert), []byte(config.DistributionKey),
	)
	if err != nil {
		return fmt.Errorf("cannot decode distribution PKI: %w", err)
	}

	if err = pki.ValidateCertWithCAChain(authPKI.Cert, ca); err != nil {
		return fmt.Errorf("error validating Auth certificate: %w", err)
	}

	if err = pki.ValidateCertWithCAChain(distributionPKI.Cert, ca); err != nil {
		return fmt.Errorf("error validating Distribution certificate: %w", err)
	}

	for _, host := range hosts {
		if err = authPKI.Cert.VerifyHostname(host); err != nil {
			return fmt.Errorf("hostname \"%v\" not supported by Auth certificate: %w", host, err)
		}

		if err = distributionPKI.Cert.VerifyHostname(host); err != nil {
			return fmt.Errorf("hostname \"%v\" not supported by Distribution certificate: %w", host, err)
		}
	}

	nc.Auth = &authPKI
	nc.Distribution = &distributionPKI

	return nil
}

func (nc *nodePKI) Process(log go_hook.Logger, ca pki.CertKey, nodeName, nodeIP string, config nodeservices.Config) error {
	certHosts := []string{
		"127.0.0.1",
		"localhost",
		nodeIP,
		helpers.RegistryServiceDNSName,
	}

	log = log.
		With("action", "ProcessNodePKI").
		With("node", nodeName)

	if config.DistributionCert != "" && config.AuthCert != "" {
		err := nc.loadFromConfig(config, ca.Cert, certHosts)
		if err == nil {
			return nil
		}
		log.Warn("Error decode Node PKI", "error", err)
	}

	log.Info("Generating new Node PKI")
	err := nc.generate(ca, certHosts)
	if err != nil {
		return fmt.Errorf("cannot generate new PKI: %w", err)
	}

	return nil
}
