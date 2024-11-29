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
