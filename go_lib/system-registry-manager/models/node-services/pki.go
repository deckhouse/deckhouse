/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

// PKI holds the configuration for the PKI
type PKI struct {
	CACert                 string `json:"ca,omitempty" yaml:"ca,omitempty"`
	AuthCert               string `json:"auth_cert,omitempty" yaml:"auth_cert,omitempty"`
	AuthKey                string `json:"auth_key,omitempty" yaml:"auth_key,omitempty"`
	TokenCert              string `json:"token_cert,omitempty" yaml:"token_cert,omitempty"`
	TokenKey               string `json:"token_key,omitempty" yaml:"token_key,omitempty"`
	DistributionCert       string `json:"distribution_cert,omitempty" yaml:"distribution_cert,omitempty"`
	DistributionKey        string `json:"distribution_key,omitempty" yaml:"distribution_key,omitempty"`
	UpstreamRegistryCACert string `json:"upstream_registry_ca,omitempty" yaml:"upstream_registry_ca,omitempty"`
	IngressClientCACert    string `json:"ingress_client_ca,omitempty" yaml:"ingress_client_ca,omitempty"`
}

func (p PKI) Validate() error {
	err := validation.ValidateStruct(&p,
		validation.Field(&p.CACert, validation.Required),
		validation.Field(&p.AuthCert, validation.Required),
		validation.Field(&p.AuthKey, validation.Required),
		validation.Field(&p.TokenCert, validation.Required),
		validation.Field(&p.TokenKey, validation.Required),
		validation.Field(&p.DistributionCert, validation.Required),
		validation.Field(&p.DistributionKey, validation.Required),
		// UpstreamRegistryCACert is optional field and can be empty
		// IngressClientCACert is optional field and can be empty
	)

	if err != nil {
		return err
	}

	caCert, err := pki.DecodeCertificate([]byte(p.CACert))
	if err != nil {
		return fmt.Errorf("cannot decode CA: %w", err)
	}

	tokenPKI, err := pki.DecodeCertKey([]byte(p.TokenCert), []byte(p.TokenKey))
	if err != nil {
		return fmt.Errorf("cannot decode Token: %w", err)
	}

	authPKI, err := pki.DecodeCertKey([]byte(p.AuthCert), []byte(p.AuthKey))
	if err != nil {
		return fmt.Errorf("cannot decode Auth: %w", err)
	}

	distributionPKI, err := pki.DecodeCertKey([]byte(p.DistributionCert), []byte(p.DistributionKey))
	if err != nil {
		return fmt.Errorf("cannot decode Distribution: %w", err)
	}

	err = pki.ValidateCertWithCAChain(tokenPKI.Cert, caCert)
	if err != nil {
		return fmt.Errorf("cannot validate Token certificate with CA: %w", err)
	}

	err = pki.ValidateCertWithCAChain(authPKI.Cert, caCert)
	if err != nil {
		return fmt.Errorf("cannot validate Auth certificate with CA: %w", err)
	}

	err = pki.ValidateCertWithCAChain(distributionPKI.Cert, caCert)
	if err != nil {
		return fmt.Errorf("cannot validate Distribution certificate with CA: %w", err)
	}

	return nil
}
