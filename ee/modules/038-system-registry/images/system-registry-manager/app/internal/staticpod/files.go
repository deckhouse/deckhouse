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

	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/node-services"
)

// syncPKIFiles synchronizes the PKI-related files in the specified directory.
// This includes saving new files, updating existing ones, and removing obsolete files,
// while updating hashes in ConfigHashes if they change.
func syncPKIFiles(basePath string, pki nodeservices.PKIModel) (bool, string, error) {
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
