/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type templateModel struct {
	Config
	Images  images
	Version string
	Address string
	Hash    string
}

type images struct {
	Distribution string
	Auth         string
	Mirrorer     string
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
func (model *templateModel) syncPKIFiles(basePath string) (bool, string, error) {
	anyFileChanged := false
	pki := model.PKI

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

// changesModel represents a model to track applied changes
type changesModel struct {
	Distribution bool `json:",omitempty"` // Indicates changes in the distribution configuration.
	Auth         bool `json:",omitempty"` // Indicates changes in the authentication system.
	PKI          bool `json:",omitempty"` // Indicates changes in the public key infrastructure.
	Pod          bool `json:",omitempty"` // Indicates changes in the pod setup.
	Mirrorer     bool `json:",omitempty"` // Indicates changes in the mirrorer configuration.
}
