/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
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
	AuthCert               string `json:"authCert,omitempty" yaml:"auth_cert,omitempty"`
	AuthKey                string `json:"authKey,omitempty" yaml:"auth_key,omitempty"`
	TokenCert              string `json:"tokenCert,omitempty" yaml:"token_cert,omitempty"`
	TokenKey               string `json:"tokenKey,omitempty" yaml:"token_key,omitempty"`
	DistributionCert       string `json:"distributionCert,omitempty" yaml:"distribution_cert,omitempty"`
	DistributionKey        string `json:"distributionKey,omitempty" yaml:"distribution_key,omitempty"`
	UpstreamRegistryCACert string `json:"upstreamRegistryCACert,omitempty" yaml:"upstream_registry_ca,omitempty"`
	IngressClientCACert    string `json:"ingressClientCACert,omitempty" yaml:"ingress_client_ca,omitempty"`
}

func (p PKIModel) Validate() error {
	return validation.ValidateStruct(&p,
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
}

// RegistryConfig holds detailed configuration of the registry
type RegistryConfig struct {
	UserRW     User              `json:"userRW,omitempty" yaml:"user_rw,omitempty"`
	UserRO     User              `json:"userRO,omitempty" yaml:"user_ro,omitempty"`
	Upstream   *UpstreamRegistry `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	HttpSecret string            `json:"httpSecret,omitempty" yaml:"http_secret,omitempty"`
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
	PasswordHash string `json:"passwordHash" yaml:"password_hash"`
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
	NoProxy string `json:"noProxy,omitempty" yaml:"no_proxy,omitempty"`
}

func (p Proxy) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Http, validation.Required),
		validation.Field(&p.Https, validation.Required),
		validation.Field(&p.NoProxy, validation.Required),
	)
}

type Mirrorer struct {
	UserPuller User     `json:"userPuller,omitempty" yaml:"user_puller,omitempty"`
	UserPusher User     `json:"userPusher,omitempty" yaml:"user_pusher,omitempty"`
	Upstreams  []string `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`
}

func (m Mirrorer) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.UserPuller, validation.Required),
		validation.Field(&m.UserPusher, validation.Required),
	)
}
