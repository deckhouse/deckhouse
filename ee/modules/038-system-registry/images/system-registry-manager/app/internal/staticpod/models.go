/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"fmt"
	"net/http"
	"path/filepath"

	validation "github.com/go-ozzo/ozzo-validation"
)

type templateModel struct {
	Config
	Address string
	Hashes  ConfigHashes
}

// Config represents the configuration
type Config struct {
	Registry RegistryConfig `json:"registry,omitempty"`
	Images   Images         `json:"images,omitempty"`
	PKI      PKIModel       `json:"pki,omitempty"`
	Proxy    *Proxy         `json:"proxy,omitempty"`
}

func (config *Config) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Registry, validation.Required),
		validation.Field(&config.Images, validation.Required),
		validation.Field(&config.PKI, validation.Required),
		validation.Field(&config.Proxy),
	)
}

func (cfg *Config) Bind(r *http.Request) error {
	return cfg.Validate()
}

// PKIModel holds the configuration for the PKI
type PKIModel struct {
	CACert           string `json:"ca,omitempty"`
	AuthCert         string `json:"authCert,omitempty"`
	AuthKey          string `json:"authKey,omitempty"`
	TokenCert        string `json:"tokenCert,omitempty"`
	TokenKey         string `json:"tokenKey,omitempty"`
	DistributionCert string `json:"distributionCert,omitempty"`
	DistributionKey  string `json:"distributionKey,omitempty"`
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
	)
}

// ConfigHashes holds the hash of the configuration files
type ConfigHashes struct {
	AuthTemplate         string
	DistributionTemplate string
	CACert               string
	AuthCert             string
	AuthKey              string
	TokenCert            string
	TokenKey             string
	DistributionCert     string
	DistributionKey      string
}

type RegistryMode string

const (
	RegistryModeDirect   RegistryMode = "Direct"
	RegistryModeProxy    RegistryMode = "Proxy"
	RegistryModeDetached RegistryMode = "Detached"
)

// RegistryConfig holds detailed configuration of the registry
type RegistryConfig struct {
	UserRW     User             `json:"userRW,omitempty"`
	UserRO     User             `json:"userRO,omitempty"`
	Mode       RegistryMode     `json:"mode,omitempty"`
	Upstream   UpstreamRegistry `json:"upstream,omitempty"`
	HttpSecret string           `json:"httpSecret,omitempty"`
}

func (rd RegistryConfig) Validate() error {
	var fields []*validation.FieldRules

	fields = append(fields, validation.Field(&rd.Mode, validation.Required))
	fields = append(fields, validation.Field(&rd.HttpSecret, validation.Required))
	fields = append(fields, validation.Field(&rd.UserRO))
	fields = append(fields, validation.Field(&rd.UserRW))

	if rd.Mode == RegistryModeProxy {
		fields = append(fields, validation.Field(&rd.Upstream))
	}

	return validation.ValidateStruct(&rd, fields...)
}

// User represents a user with a name and a password hash
type User struct {
	Name         string `json:"name"`
	PasswordHash string `json:"passwordHash"`
}

func (u User) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Name, validation.Required),
		validation.Field(&u.PasswordHash, validation.Required),
	)
}

// UpstreamRegistry holds upstream registry configuration details
type UpstreamRegistry struct {
	Scheme   string  `json:"scheme,omitempty"`
	Host     string  `json:"host,omitempty"`
	Path     string  `json:"path,omitempty"`
	CA       string  `json:"ca,omitempty"`
	User     string  `json:"user,omitempty"`
	Password string  `json:"password,omitempty"`
	TTL      *string `json:"ttl,omitempty"`
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
	Distribution string `json:"distribution,omitempty"`
	Auth         string `json:"auth,omitempty"`
	Mirrorer     string `json:"mirrorer,omitempty"`
}

func (im Images) Validate() error {
	return validation.ValidateStruct(&im,
		validation.Field(&im.Auth, validation.Required),
		validation.Field(&im.Distribution, validation.Required),
		validation.Field(&im.Mirrorer, validation.Required),
	)
}

type Proxy struct {
	Http    string `json:"http,omitempty"`
	Https   string `json:"https,omitempty"`
	NoProxy string `json:"noProxy,omitempty"`
}

func (p Proxy) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Http, validation.Required),
		validation.Field(&p.Https, validation.Required),
		validation.Field(&p.NoProxy, validation.Required),
	)
}

// processTemplate processes the given template file and saves the rendered result to the specified path
func (config *templateModel) processTemplate(name templateName, outputPath string, hashField *string) (bool, error) {
	// Render the template with the given configuration
	renderedContent, err := renderTemplate(name, config)
	if err != nil {
		return false, fmt.Errorf("failed to render template %s: %v", name, err)
	}

	// Compute the hash of the rendered content
	hash := computeHash(renderedContent)

	// Update the hashField if provided
	if hashField != nil {
		*hashField = hash
	}

	// Compare the existing file's content with the new rendered content
	if isSame, err := compareFileHash(outputPath, renderedContent); err != nil {
		return false, fmt.Errorf("failed to compare file hash for %s: %v", outputPath, err)
	} else if isSame {
		return false, nil
	}

	// Save the new content to the file
	if err := saveToFile(renderedContent, outputPath); err != nil {
		return false, fmt.Errorf("failed to save file %s: %v", outputPath, err)
	}

	return true, nil
}

// savePKIFiles saves the PKI-related files to the specified directory and updates hashes in ConfigHashes if they change
func (pki *PKIModel) savePKIFiles(basePath string, configHashes *ConfigHashes) (bool, error) {
	anyFileChanged := false

	// Define paths for each PKI file and corresponding hash field in ConfigHashes
	fileMap := map[string]struct {
		content   string
		hashField *string
	}{
		"ca.crt":           {pki.CACert, &configHashes.CACert},
		"auth.crt":         {pki.AuthCert, &configHashes.AuthCert},
		"auth.key":         {pki.AuthKey, &configHashes.AuthKey},
		"token.crt":        {pki.TokenCert, &configHashes.TokenCert},
		"token.key":        {pki.TokenKey, &configHashes.TokenKey},
		"distribution.crt": {pki.DistributionCert, &configHashes.DistributionCert},
		"distribution.key": {pki.DistributionKey, &configHashes.DistributionKey},
	}

	// Iterate over the PKI files and process them
	for name, data := range fileMap {
		path := filepath.Join(basePath, name)

		// Process each template and check if it has changed
		changed, err := saveFileIfChanged(path, []byte(data.content), data.hashField)
		if err != nil {
			return false, fmt.Errorf("failed to process PKI file %s: %v", path, err)
		}

		anyFileChanged = anyFileChanged || changed
	}

	return anyFileChanged, nil
}
