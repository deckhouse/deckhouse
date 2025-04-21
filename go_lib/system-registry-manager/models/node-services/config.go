/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

// Config represents the configuration
type Config struct {
	CACert           string `json:"ca,omitempty" yaml:"ca"`
	AuthCert         string `json:"auth_cert" yaml:"auth_cert"`
	AuthKey          string `json:"auth_key" yaml:"auth_key"`
	TokenCert        string `json:"token_cert" yaml:"token_cert"`
	TokenKey         string `json:"token_key" yaml:"token_key"`
	DistributionCert string `json:"distribution_cert" yaml:"distribution_cert"`
	DistributionKey  string `json:"distribution_key" yaml:"distribution_key"`

	UserRO     User   `json:"user_ro" yaml:"user_ro"`
	HTTPSecret string `json:"http_secret" yaml:"http_secret"`

	LocalMode *LocalMode `json:"local_mode,omitempty" yaml:"local_mode,omitempty"`
	ProxyMode *ProxyMode `json:"proxy_mode,omitempty" yaml:"proxy_mode,omitempty"`

	ProxyConfig *ProxyConfig `json:"proxy_config,omitempty" yaml:"proxy,omitempty"`
}

func (config *Config) Validate() error {
	err := validation.ValidateStruct(config,
		validation.Field(&config.CACert, validation.Required),
		validation.Field(&config.AuthCert, validation.Required),
		validation.Field(&config.AuthKey, validation.Required),
		validation.Field(&config.TokenCert, validation.Required),
		validation.Field(&config.TokenKey, validation.Required),
		validation.Field(&config.DistributionCert, validation.Required),
		validation.Field(&config.DistributionKey, validation.Required),

		validation.Field(&config.HTTPSecret, validation.Required),
		validation.Field(&config.UserRO, validation.Required),

		validation.Field(&config.ProxyConfig),

		validation.Field(&config.LocalMode),
		validation.Field(&config.ProxyMode),
	)

	if err != nil {
		return err
	}

	caCert, err := pki.DecodeCertificate([]byte(config.CACert))
	if err != nil {
		return fmt.Errorf("cannot decode CA: %w", err)
	}

	tokenPKI, err := pki.DecodeCertKey([]byte(config.TokenCert), []byte(config.TokenKey))
	if err != nil {
		return fmt.Errorf("cannot decode Token: %w", err)
	}

	authPKI, err := pki.DecodeCertKey([]byte(config.AuthCert), []byte(config.AuthKey))
	if err != nil {
		return fmt.Errorf("cannot decode Auth: %w", err)
	}

	distributionPKI, err := pki.DecodeCertKey([]byte(config.DistributionCert), []byte(config.DistributionKey))
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

	if config.ProxyMode == nil && config.LocalMode == nil {
		return errors.New("one mode field should be filled")
	}

	if config.ProxyMode != nil && config.LocalMode != nil {
		return errors.New("only one mode field should be filled")
	}

	return nil
}
