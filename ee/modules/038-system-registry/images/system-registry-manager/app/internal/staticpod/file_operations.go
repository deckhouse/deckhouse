/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// EmbeddedRegistryConfig represents the configuration for the registry
type EmbeddedRegistryConfig struct {
	IpAddress    string
	Registry     RegistryDetails
	Images       Images
	ConfigHashes ConfigHashes
	Pki          Pki
	Proxy        *Proxy
}

// Pki holds the configuration for the PKI
type Pki struct {
	CaCert           string
	AuthCert         string
	AuthKey          string
	AuthTokenCert    string
	AuthTokenKey     string
	DistributionCert string
	DistributionKey  string
}

// ConfigHashes holds the hash of the configuration files
type ConfigHashes struct {
	AuthTemplateHash         string
	DistributionTemplateHash string
	CaCertHash               string
	AuthCertHash             string
	AuthKeyHash              string
	AuthTokenCertHash        string
	AuthTokenKeyHash         string
	DistributionCertHash     string
	DistributionKeyHash      string
}

// RegistryDetails holds detailed configuration of the registry
type RegistryDetails struct {
	UserRw           User
	UserRo           User
	RegistryMode     string
	UpstreamRegistry UpstreamRegistry
	HttpSecret       string
}

// User represents a user with a name and a password hash
type User struct {
	Name         string
	PasswordHash string
}

// UpstreamRegistry holds upstream registry configuration details
type UpstreamRegistry struct {
	Scheme   string
	Host     string
	Path     string
	CA       string
	User     string
	Password string
	TTL      *string
}

type Images struct {
	DockerDistribution string
	DockerAuth         string
}

type Proxy struct {
	HttpProxy  string
	HttpsProxy string
	NoProxy    string
}

// processTemplate processes the given template file and saves the rendered result to the specified path
func (config *EmbeddedRegistryConfig) processTemplate(name templateName, outputPath string, hashField *string) (bool, error) {
	// Read the template file content
	templateBytes, err := getTemplateContent(name)
	if err != nil {
		return false, fmt.Errorf("failed to read template %s: %v", name, err)
	}

	// Render the template with the given configuration
	renderedContent, err := renderTemplate(string(templateBytes), config)
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

// RenderTemplate renders the provided template content with the given data
func renderTemplate(templateContent string, data interface{}) ([]byte, error) {
	funcMap := template.FuncMap{
		"quote": func(s string) string { return strconv.Quote(s) },
	}

	tmpl, err := template.New("template").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("error executing template: %v", err)
	}

	return buf.Bytes(), nil
}

// SaveToFile saves the rendered content to the specified file path
func saveToFile(content []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error creating directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, content, 0600); err != nil {
		return fmt.Errorf("error writing to file %s: %v", path, err)
	}

	return nil
}

// deleteFile deletes the file at the specified path
func deleteFile(path string) (bool, error) {

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("error deleting file %s: %v", path, err)
	}

	return true, nil
}

// deleteDirectory deletes the directory at the specified path
func deleteDirectory(path string) (bool, error) {

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}

	if err := os.RemoveAll(path); err != nil {
		return false, fmt.Errorf("error deleting directory %s: %v", path, err)
	}

	return true, nil
}

func (config *EmbeddedRegistryConfig) validate() error {
	var missingFields []string

	// Validate IP address
	if config.IpAddress == "" {
		missingFields = append(missingFields, "IpAddress")
	}

	// Validate registry users
	if config.Registry.UserRw.Name == "" {
		missingFields = append(missingFields, "UserRw.Name")
	}
	if config.Registry.UserRw.PasswordHash == "" {
		missingFields = append(missingFields, "UserRw.PasswordHash")
	}
	if config.Registry.UserRo.Name == "" {
		missingFields = append(missingFields, "UserRo.Name")
	}
	if config.Registry.UserRo.PasswordHash == "" {
		missingFields = append(missingFields, "UserRo.PasswordHash")
	}

	// Validate registry mode and upstream registry
	if config.Registry.RegistryMode == "" {
		missingFields = append(missingFields, "RegistryMode")
	}
	if config.Registry.RegistryMode == "Proxy" {
		if config.Registry.UpstreamRegistry.Scheme == "" {
			missingFields = append(missingFields, "UpstreamRegistry.Scheme")
		}
		if config.Registry.UpstreamRegistry.Host == "" {
			missingFields = append(missingFields, "UpstreamRegistry.Host")
		}
		if config.Registry.UpstreamRegistry.Path == "" {
			missingFields = append(missingFields, "UpstreamRegistry.Path")
		}
		if config.Registry.UpstreamRegistry.User == "" {
			missingFields = append(missingFields, "UpstreamRegistry.User")
		}
		if config.Registry.UpstreamRegistry.Password == "" {
			missingFields = append(missingFields, "UpstreamRegistry.Password")
		}
	}

	// Validate registry http secret
	if config.Registry.HttpSecret == "" {
		missingFields = append(missingFields, "Registry.HttpSecret")
	}

	// Validate images
	if config.Images.DockerDistribution == "" {
		missingFields = append(missingFields, "Images.DockerDistribution")
	}
	if config.Images.DockerAuth == "" {
		missingFields = append(missingFields, "Images.DockerAuth")
	}

	// Validate node PKI
	if config.Pki.CaCert == "" {
		missingFields = append(missingFields, "Pki.CaCert")
	}
	if config.Pki.AuthCert == "" {
		missingFields = append(missingFields, "Pki.AuthCert")
	}
	if config.Pki.AuthKey == "" {
		missingFields = append(missingFields, "Pki.AuthKey")
	}
	if config.Pki.AuthTokenCert == "" {
		missingFields = append(missingFields, "Pki.AuthTokenCert")
	}
	if config.Pki.AuthTokenKey == "" {
		missingFields = append(missingFields, "Pki.AuthTokenCert")
	}
	if config.Pki.DistributionCert == "" {
		missingFields = append(missingFields, "Pki.DistributionCert")
	}
	if config.Pki.DistributionKey == "" {
		missingFields = append(missingFields, "Pki.DistributionKey")
	}

	// Validate proxy if present
	if config.Proxy != nil {
		if config.Proxy.HttpProxy == "" {
			missingFields = append(missingFields, "Proxy.HttpProxy")
		}
		if config.Proxy.HttpsProxy == "" {
			missingFields = append(missingFields, "Proxy.HttpsProxy")
		}
		if config.Proxy.NoProxy == "" {
			missingFields = append(missingFields, "Proxy.NoProxy")
		}
	}

	// If there are missing fields, return an error
	if len(missingFields) > 0 {
		return fmt.Errorf("validation error, missing fields: %s", strings.Join(missingFields, ", "))
	}

	return nil
}

// fillHostIpAddress fills the IpAddress struct field with the HOST_IP environment variable
func (config *EmbeddedRegistryConfig) fillHostIpAddress() (string, error) {
	if hostIP := os.Getenv("HOST_IP"); hostIP != "" {
		return hostIP, nil
	}
	return "", fmt.Errorf("HOST_IP environment variable is not set")
}

// computeHash computes the SHA-256 hash of the given content.
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// compareFileHash reads the file at the given path and compares its hash with the provided new content.
func compareFileHash(path string, newContent []byte) (bool, error) {
	currentContent, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// File doesn't exist, so consider it different
		return false, nil
	} else if err != nil {
		return false, err
	}

	// Compute hashes for both the current file content and new content
	currentHash := computeHash(currentContent)
	newHash := computeHash(newContent)

	// Return whether the hashes match
	return currentHash == newHash, nil
}

// savePkiFiles saves the PKI-related files to the specified directory and updates hashes in ConfigHashes if they change
func (pki *Pki) savePkiFiles(basePath string, configHashes *ConfigHashes) (bool, error) {
	anyFileChanged := false

	// Define paths for each PKI file and corresponding hash field in ConfigHashes
	fileMap := map[string]struct {
		content   string
		hashField *string
	}{
		"ca.crt":           {pki.CaCert, &configHashes.CaCertHash},
		"auth.crt":         {pki.AuthCert, &configHashes.AuthCertHash},
		"auth.key":         {pki.AuthKey, &configHashes.AuthKeyHash},
		"token.crt":        {pki.AuthTokenCert, &configHashes.AuthTokenCertHash},
		"token.key":        {pki.AuthTokenKey, &configHashes.AuthTokenKeyHash},
		"distribution.crt": {pki.DistributionCert, &configHashes.DistributionCertHash},
		"distribution.key": {pki.DistributionKey, &configHashes.DistributionKeyHash},
	}

	// Iterate over the PKI files and process them
	for filename, fileData := range fileMap {
		path := filepath.Join(basePath, filename)

		// Process each template and check if it has changed
		changed, err := processTemplateForFile(path, []byte(fileData.content), fileData.hashField)
		if err != nil {
			return false, fmt.Errorf("failed to process PKI file %s: %v", path, err)
		}

		anyFileChanged = anyFileChanged || changed
	}

	return anyFileChanged, nil
}

// processTemplateForFile processes the content, compares it with the existing file, and updates the hash field
func processTemplateForFile(outputPath string, content []byte, hashField *string) (bool, error) {
	// Compute the hash of the new content
	hash := computeHash(content)

	// Update the hash field if provided
	if hashField != nil {
		*hashField = hash
	}

	// Compare the existing file content with the new content
	isSame, err := compareFileHash(outputPath, content)
	if err != nil {
		return false, fmt.Errorf("failed to compare file content for %s: %v", outputPath, err)
	}

	// If the content is the same, no need to overwrite the file
	if isSame {
		return false, nil
	}

	// Save the new content to the file
	if err := saveToFile(content, outputPath); err != nil {
		return false, fmt.Errorf("failed to save file %s: %v", outputPath, err)
	}

	return true, nil
}
