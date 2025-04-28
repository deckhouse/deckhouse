/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
