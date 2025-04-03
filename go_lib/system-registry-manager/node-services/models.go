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

// Config represents the configuration
type Config struct {
	Registry RegistryConfig `json:"registry,omitempty" yaml:"registry,omitempty"`
	PKI      PKIModel       `json:"pki,omitempty" yaml:"pki,omitempty"`
	Proxy    *Proxy         `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}

func (config *Config) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Registry, validation.Required),
		validation.Field(&config.PKI, validation.Required),
		validation.Field(&config.Proxy),
	)
}

// PKIModel holds the configuration for the PKI
type PKIModel struct {
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

func (p PKIModel) Validate() error {
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

// RegistryConfig holds detailed configuration of the registry
type RegistryConfig struct {
	UserRW     User              `json:"user_rw,omitempty" yaml:"user_rw,omitempty"`
	UserRO     User              `json:"user_ro,omitempty" yaml:"user_ro,omitempty"`
	Upstream   *UpstreamRegistry `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	HttpSecret string            `json:"http_secret,omitempty" yaml:"http_secret,omitempty"`
	Mirrorer   *Mirrorer         `json:"mirrorer,omitempty" yaml:"mirrorer,omitempty"`
}

func (rd RegistryConfig) Validate() error {
	var fields []*validation.FieldRules

	fields = append(fields, validation.Field(&rd.HttpSecret, validation.Required))
	fields = append(fields, validation.Field(&rd.UserRO, validation.Required))
	fields = append(fields, validation.Field(&rd.UserRW, validation.Required))

	fields = append(fields, validation.Field(&rd.Mirrorer))
	fields = append(fields, validation.Field(&rd.Upstream))

	return validation.ValidateStruct(&rd, fields...)
}

// User represents a user with a name and a password hash
type User struct {
	Name         string `json:"name" yaml:"name"`
	Password     string `json:"password" yaml:"password"`
	PasswordHash string `json:"password_hash" yaml:"password_hash"`
}

func (u User) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Name, validation.Required),
		validation.Field(&u.Password, validation.Required),
		validation.Field(&u.PasswordHash, validation.Required),
	)
}

// UpstreamRegistry holds upstream registry configuration details
type UpstreamRegistry struct {
	Scheme   string  `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	Host     string  `json:"host,omitempty" yaml:"host,omitempty"`
	Path     string  `json:"path,omitempty" yaml:"path,omitempty"`
	User     string  `json:"user,omitempty" yaml:"user,omitempty"`
	Password string  `json:"password,omitempty" yaml:"password,omitempty"`
	TTL      *string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

func (u UpstreamRegistry) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Scheme, validation.Required),
		validation.Field(&u.Host, validation.Required),
		validation.Field(&u.Path, validation.Required),
		validation.Field(&u.User, validation.Required),
		validation.Field(&u.Password, validation.Required),
	)
}

type Proxy struct {
	Http    string `json:"http,omitempty" yaml:"http,omitempty"`
	Https   string `json:"https,omitempty" yaml:"https,omitempty"`
	NoProxy string `json:"no_proxy,omitempty" yaml:"no_proxy,omitempty"`
}

func (p Proxy) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Http, validation.Required),
		validation.Field(&p.Https, validation.Required),
		validation.Field(&p.NoProxy, validation.Required),
	)
}

type Mirrorer struct {
	UserPuller User     `json:"user_puller,omitempty" yaml:"user_puller,omitempty"`
	UserPusher User     `json:"user_pusher,omitempty" yaml:"user_pusher,omitempty"`
	Upstreams  []string `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`
}

func (m Mirrorer) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.UserPuller, validation.Required),
		validation.Field(&m.UserPusher, validation.Required),
	)
}
