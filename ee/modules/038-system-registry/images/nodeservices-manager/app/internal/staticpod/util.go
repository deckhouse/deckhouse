/*
Copyright 2025 Flant JSC

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

package staticpod

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// computeHash computes the SHA-256 hash of the given content.
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// compareFileHash reads the file at the given path and compares its hash with the provided new content.
func compareFileHash(path string, newContent []byte) (bool, string, error) {
	newHash := computeHash(newContent)

	currentContent, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// File doesn't exist, so consider it different
		return false, newHash, nil
	} else if err != nil {
		return false, newHash, err
	}

	currentHash := computeHash(currentContent)

	// Return whether the hashes match
	return currentHash == newHash, newHash, nil
}

// saveFileIfChanged computing content's hash, compares it with the existing file hash and overwrites file if it different
// hash is updated with actual value
func saveFileIfChanged(outputPath string, content []byte) (bool, string, error) {
	// Compare the existing file content with the new content
	isSame, hash, err := compareFileHash(outputPath, content)
	if err != nil {
		return false, hash, fmt.Errorf("failed to compare file content for %s: %v", outputPath, err)
	}

	// If the content is the same, no need to overwrite the file
	if isSame {
		return false, hash, nil
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, hash, fmt.Errorf("error creating directory %s: %v", dir, err)
	}

	if err := os.WriteFile(outputPath, content, 0600); err != nil {
		return false, hash, fmt.Errorf("error writing to file %s: %v", outputPath, err)
	}

	return true, hash, nil
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
