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

	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/node-services"
)

// syncPKIFiles synchronizes the PKI-related files in the specified directory.
// This includes saving new files, updating existing ones, and removing obsolete files,
// while updating hashes in ConfigHashes if they change.
func syncPKIFiles(basePath string, config nodeservices.Config) (bool, string, error) {
	anyFileChanged := false

	// Define paths for each PKI file and corresponding hash field in ConfigHashes
	fileMap := map[string]string{
		"ca.crt":           config.CACert,
		"auth.crt":         config.AuthCert,
		"auth.key":         config.AuthKey,
		"token.crt":        config.TokenCert,
		"token.key":        config.TokenKey,
		"distribution.crt": config.DistributionCert,
		"distribution.key": config.DistributionKey,
	}

	if config.LocalMode != nil {
		fileMap["ingress-client-ca.crt"] = config.LocalMode.IngressClientCACert
	} else {
		fileMap["ingress-client-ca.crt"] = ""
	}

	if config.ProxyMode != nil {
		fileMap["upstream-registry-ca.crt"] = config.ProxyMode.UpstreamRegistryCACert
	} else {
		fileMap["upstream-registry-ca.crt"] = ""
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
