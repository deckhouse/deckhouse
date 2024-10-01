package static_pod

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
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
}

// ConfigHashes holds the hash of the configuration files
type ConfigHashes struct {
	AuthTemplateHash         string
	DistributionTemplateHash string
}

// RegistryDetails holds detailed configuration of the registry
type RegistryDetails struct {
	UserRw           User
	UserRo           User
	RegistryMode     string
	UpstreamRegistry UpstreamRegistry
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
	CA       string
	User     string
	Password string
}

type Images struct {
	DockerDistribution string
	DockerAuth         string
}

// processTemplate processes the given template file and saves the rendered result to the specified path
func (config *EmbeddedRegistryConfig) processTemplate(templatePath, outputPath string, hashField *string) (bool, error) {
	// Read the template file content
	templateContent, err := readTemplate(templatePath)
	if err != nil {
		return false, fmt.Errorf("failed to read template file %s: %v", templatePath, err)
	}

	// Render the template with the given configuration
	renderedContent, err := renderTemplate(templateContent, config)
	if err != nil {
		return false, fmt.Errorf("failed to render template %s: %v", templatePath, err)
	}

	// Compute the hash of the rendered content
	hash, err := computeHash(renderedContent)
	if err != nil {
		return false, fmt.Errorf("failed to compute hash for template %s: %v", templatePath, err)
	}

	// Update the hashField if provided
	if hashField != nil {
		*hashField = hash
	}

	// Compare the existing file's content with the new rendered content
	isSame, err := compareFileHash(outputPath, renderedContent)
	if err != nil {
		return false, fmt.Errorf("failed to compare file hash for %s: %v", outputPath, err)
	}

	// If the content is the same, no need to overwrite the file
	if isSame {
		return false, nil
	}

	// Save the new content to the file
	if err := saveToFile(renderedContent, outputPath); err != nil {
		return false, fmt.Errorf("failed to save file %s: %v", outputPath, err)
	}

	return true, nil
}

// ReadTemplate reads the template content from the given file path
func readTemplate(path string) (string, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

// RenderTemplate renders the provided template content with the given data
func renderTemplate(templateContent string, data interface{}) (string, error) {
	funcMap := template.FuncMap{
		"quote":      func(s string) string { return strconv.Quote(s) },
		"trimSuffix": strings.TrimSuffix,
		"trimPrefix": strings.TrimPrefix,
	}

	tmpl, err := template.New("template").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("error parsing template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error executing template: %v", err)
	}

	return buf.String(), nil
}

// SaveToFile saves the rendered content to the specified file path
func saveToFile(content string, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
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

func (config *EmbeddedRegistryConfig) validate() error {
	var missingFields []string

	// ip address to bind to
	if config.IpAddress == "" {
		missingFields = append(missingFields, "IpAddress")
	}

	// Check rw and ro users
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

	// check registry mode, if Proxy, check upstream registry
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
		if config.Registry.UpstreamRegistry.User == "" {
			missingFields = append(missingFields, "UpstreamRegistry.User")
		}
		if config.Registry.UpstreamRegistry.Password == "" {
			missingFields = append(missingFields, "UpstreamRegistry.Password")
		}
	}

	// Images
	if config.Images.DockerDistribution == "" {
		missingFields = append(missingFields, "Images.DockerDistribution")
	}
	if config.Images.DockerAuth == "" {
		missingFields = append(missingFields, "Images.DockerAuth")
	}

	// If there are missing fields, return an error
	if len(missingFields) > 0 {
		return fmt.Errorf("error, missing required fields: %s", strings.Join(missingFields, ", "))
	}

	return nil
}

func (config *EmbeddedRegistryConfig) fillHostIpAddress() (string, error) {
	hostIP := os.Getenv("HOST_IP")
	if hostIP == "" {
		return "", fmt.Errorf("HOST_IP environment variable is not set")
	}
	return hostIP, nil
}

// computeHash computes the SHA-256 hash of the given content.
func computeHash(content string) (string, error) {
	hash := sha256.New()
	_, err := hash.Write([]byte(content))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// compareFileHash reads the file at the given path and compares its hash with the provided hash.
func compareFileHash(path, newContent string) (bool, error) {
	currentContent, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		// If the file doesn't exist, treat it as different
		return false, nil
	} else if err != nil {
		return false, err
	}

	currentHash, err := computeHash(string(currentContent))
	if err != nil {
		return false, err
	}

	newHash, err := computeHash(newContent)
	if err != nil {
		return false, err
	}

	return currentHash == newHash, nil
}
