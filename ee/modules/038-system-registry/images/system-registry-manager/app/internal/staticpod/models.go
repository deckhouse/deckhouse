/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
)

type templateModel struct {
	Config
	Images  Images
	Version string
	Address string
	Hash    string
}

type NodeServicesConfigModel struct {
	Version string `json:"version"`
	Config  Config `json:"config"`
}

func (config *NodeServicesConfigModel) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Config, validation.Required),
	)
}

func (cfg *NodeServicesConfigModel) Bind(r *http.Request) error {
	return cfg.Validate()
}

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

type Images struct {
	Distribution string
	Auth         string
	Mirrorer     string
}

func (im Images) Validate() error {
	return validation.ValidateStruct(&im,
		validation.Field(&im.Auth, validation.Required),
		validation.Field(&im.Distribution, validation.Required),
		validation.Field(&im.Mirrorer, validation.Required),
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

// processTemplate processes the given template file and saves the rendered result to the specified path
func (config *templateModel) processTemplate(name templateName, outputPath string) (bool, string, error) {
	// Render the template with the given configuration
	renderedContent, err := renderTemplate(name, config)
	if err != nil {
		return false, "", fmt.Errorf("failed to render template %s: %v", name, err)
	}

	chaged, hash, err := saveFileIfChanged(outputPath, renderedContent)
	if err != nil {
		return chaged, hash, fmt.Errorf("failed to save file %s: %w", outputPath, err)
	}
	return chaged, hash, nil
}

// syncPKIFiles synchronizes the PKI-related files in the specified directory.
// This includes saving new files, updating existing ones, and removing obsolete files,
// while updating hashes in ConfigHashes if they change.
func (pki *PKIModel) syncPKIFiles(basePath string) (bool, string, error) {
	anyFileChanged := false

	// Define paths for each PKI file and corresponding hash field in ConfigHashes
	fileMap := map[string]string{
		"ca.crt":                   pki.CACert,
		"auth.crt":                 pki.AuthCert,
		"auth.key":                 pki.AuthKey,
		"token.crt":                pki.TokenCert,
		"token.key":                pki.TokenKey,
		"distribution.crt":         pki.DistributionCert,
		"distribution.key":         pki.DistributionKey,
		"ingress-client-ca.crt":    pki.IngressClientCACert,
		"upstream-registry-ca.crt": pki.UpstreamRegistryCACert,
	}

	hashes := make([]string, 0, len(fileMap))

	// Iterate over the PKI files and process them
	for name, data := range fileMap {
		path := filepath.Join(basePath, name)

		// Process each template and check if it has changed
		if data != "" {
			changed, hash, err := saveFileIfChanged(path, []byte(data))
			if err != nil {
				return false, "", fmt.Errorf("failed to process PKI file %s: %v", path, err)
			}

			hashes = append(hashes, hash)

			anyFileChanged = anyFileChanged || changed
		} else {
			changed, err := deleteFile(path)
			if err != nil {
				return false, "", fmt.Errorf("failed to process PKI file %s: %v", path, err)
			}
			anyFileChanged = anyFileChanged || changed
		}
	}

	sort.Strings(hashes)
	hashesStr := strings.Join(hashes, "\n")
	return anyFileChanged, computeHash([]byte(hashesStr)), nil
}

// ChangesModel represents a model to track applied changes
type ChangesModel struct {
	Distribution bool `json:",omitempty"` // Indicates changes in the distribution configuration.
	Auth         bool `json:",omitempty"` // Indicates changes in the authentication system.
	PKI          bool `json:",omitempty"` // Indicates changes in the public key infrastructure.
	Pod          bool `json:",omitempty"` // Indicates changes in the pod setup.
	Mirrorer     bool `json:",omitempty"` // Indicates changes in the mirrorer configuration.
}

// HasChanges checks if any field is true.
func (c ChangesModel) HasChanges() bool {
	return c.Distribution || c.Auth || c.PKI || c.Pod || c.Mirrorer
}
