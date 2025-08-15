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

package downloader

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrReleaseChannelNotFound indicates the requested release channel tag doesn't exist
	ErrReleaseChannelNotFound = errors.New("release channel not found")

	// ErrVersionNotInRegistry indicates the version exists in release channel but not in registry
	ErrVersionNotInRegistry = errors.New("version not found in registry")

	// ErrManifestNotFound indicates the manifest/image cannot be loaded
	ErrManifestNotFound = errors.New("manifest not found")
)

// ReleaseChannelError represents errors specific to release channel operations
type ReleaseChannelError struct {
	ModuleName     string
	ReleaseChannel string
	Operation      string
	Err            error
}

func (e *ReleaseChannelError) Error() string {
	return fmt.Sprintf("release channel \"%s\" for module \"%s\" %s: %v",
		e.ReleaseChannel, e.ModuleName, e.Operation, e.Err)
}

func (e *ReleaseChannelError) Unwrap() error {
	return e.Err
}

// RegistryError represents errors specific to registry operations
type RegistryError struct {
	ModuleName string
	Version    string
	Operation  string
	Err        error
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("registry error for module `%s` version `%s` %s: %v",
		e.ModuleName, e.Version, e.Operation, e.Err)
}

func (e *RegistryError) Unwrap() error {
	return e.Err
}

// classifyRegistryError analyzes registry errors and returns appropriate error types
func classifyRegistryError(err error, moduleName, version, operation string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Check for common "not found" patterns in registry errors
	notFoundPatterns := []string{
		"not found",
		"404",
		"NAME_UNKNOWN",
		"MANIFEST_UNKNOWN",
		"TAG_INVALID",
		"BLOB_UNKNOWN",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			if operation == "get digest" || operation == "get image" {
				return &RegistryError{
					ModuleName: moduleName,
					Version:    version,
					Operation:  operation,
					Err:        ErrVersionNotInRegistry,
				}
			}
			if strings.Contains(operation, "manifest") {
				return &RegistryError{
					ModuleName: moduleName,
					Version:    version,
					Operation:  operation,
					Err:        ErrManifestNotFound,
				}
			}
			return &RegistryError{
				ModuleName: moduleName,
				Version:    version,
				Operation:  operation,
				Err:        ErrVersionNotInRegistry,
			}
		}
	}

	// Return original error wrapped in RegistryError for other cases
	return &RegistryError{
		ModuleName: moduleName,
		Version:    version,
		Operation:  operation,
		Err:        err,
	}
}

// ClassifyReleaseChannelError analyzes release channel errors and returns appropriate error types
func ClassifyReleaseChannelError(err error, moduleName, releaseChannel, operation string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Check for common "not found" patterns in release channel errors
	notFoundPatterns := []string{
		"not found",
		"404",
		"NAME_UNKNOWN",
		"MANIFEST_UNKNOWN",
		"TAG_INVALID",
		"BLOB_UNKNOWN",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			return &ReleaseChannelError{
				ModuleName:     moduleName,
				ReleaseChannel: releaseChannel,
				Operation:      operation,
				Err:            ErrReleaseChannelNotFound,
			}
		}
	}

	// Return original error wrapped in ReleaseChannelError for other cases
	return &ReleaseChannelError{
		ModuleName:     moduleName,
		ReleaseChannel: releaseChannel,
		Operation:      operation,
		Err:            err,
	}
}

// IsReleaseChannelNotFoundError checks if the error indicates a missing release channel
func IsReleaseChannelNotFoundError(err error) bool {
	var releaseErr *ReleaseChannelError
	if errors.As(err, &releaseErr) {
		return errors.Is(releaseErr.Err, ErrReleaseChannelNotFound)
	}
	return false
}

// IsVersionNotInRegistryError checks if the error indicates a version not found in registry
func IsVersionNotInRegistryError(err error) bool {
	var registryErr *RegistryError
	if errors.As(err, &registryErr) {
		return errors.Is(registryErr.Err, ErrVersionNotInRegistry)
	}
	return false
}
