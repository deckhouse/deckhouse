/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controlplane

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// calculateComponentChecksum calculates checksum of component manifest including referenced files.
// This needs if referenced files (like audit-policy.yaml from kube-apiserver manifest) were changed.
func calculateComponentChecksum(manifest []byte, tmpDir string) (string, error) {
	h := sha256.New()

	if _, err := h.Write(manifest); err != nil {
		return "", fmt.Errorf("failed to hash manifest: %w", err)
	}
	re := regexp.MustCompile(`=(/etc/kubernetes/.+)`)
	matches := re.FindAllSubmatch(manifest, -1)

	filesMap := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			filesMap[string(match[1])] = struct{}{}
		}
	}

	files := make([]string, 0, len(filesMap))
	for file := range filesMap {
		files = append(files, file)
	}
	sort.Strings(files)

	for _, file := range files {
		filePath := filepath.Join(tmpDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", file, err)
		}
		if _, err := h.Write(content); err != nil {
			return "", fmt.Errorf("failed to hash file %s: %w", file, err)
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// calculatePKIChecksum calculates the total checksum of all the keys of the pki secret.
func calculatePKIChecksum(pkiSecret *corev1.Secret) (string, error) {
	h := sha256.New()

	keys := make([]string, 0, len(pkiSecret.Data))
	for key := range pkiSecret.Data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		h.Write([]byte(key))
		h.Write(pkiSecret.Data[key])
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
