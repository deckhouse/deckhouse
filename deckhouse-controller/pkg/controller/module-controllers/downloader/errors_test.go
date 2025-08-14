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
	"testing"
)

func TestClassifyReleaseChannelError(t *testing.T) {
	tests := []struct {
		name           string
		originalError  error
		moduleName     string
		releaseChannel string
		operation      string
		expectType     error
		expectMessage  string
	}{
		{
			name:           "Not found error",
			originalError:  errors.New("manifest not found"),
			moduleName:     "test-module",
			releaseChannel: "stable",
			operation:      "get digest",
			expectType:     ErrReleaseChannelNotFound,
			expectMessage:  "release channel 'stable' for module 'test-module' get digest: release channel not found",
		},
		{
			name:           "404 error",
			originalError:  errors.New("404 Not Found"),
			moduleName:     "test-module",
			releaseChannel: "beta",
			operation:      "get image",
			expectType:     ErrReleaseChannelNotFound,
			expectMessage:  "release channel 'beta' for module 'test-module' get image: release channel not found",
		},
		{
			name:           "NAME_UNKNOWN error",
			originalError:  errors.New("NAME_UNKNOWN: repository does not exist"),
			moduleName:     "test-module",
			releaseChannel: "alpha",
			operation:      "get digest",
			expectType:     ErrReleaseChannelNotFound,
			expectMessage:  "release channel 'alpha' for module 'test-module' get digest: release channel not found",
		},
		{
			name:           "Other error",
			originalError:  errors.New("connection timeout"),
			moduleName:     "test-module",
			releaseChannel: "stable",
			operation:      "get digest",
			expectType:     errors.New("connection timeout"),
			expectMessage:  "release channel 'stable' for module 'test-module' get digest: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyReleaseChannelError(tt.originalError, tt.moduleName, tt.releaseChannel, tt.operation)

			if result.Error() != tt.expectMessage {
				t.Errorf("Expected message %q, got %q", tt.expectMessage, result.Error())
			}

			var releaseErr *ReleaseChannelError
			if errors.As(result, &releaseErr) {
				if tt.expectType == tt.originalError {
					// For "Other error" case, we expect the original error to be wrapped
					if releaseErr.Err.Error() != tt.originalError.Error() {
						t.Errorf("Expected wrapped original error %q, got %q", tt.originalError.Error(), releaseErr.Err.Error())
					}
				} else if !errors.Is(releaseErr.Err, tt.expectType) {
					t.Errorf("Expected wrapped error type %T, got %T", tt.expectType, releaseErr.Err)
				}
			} else {
				t.Errorf("Expected ReleaseChannelError wrapper")
			}
		})
	}
}

func TestClassifyRegistryError(t *testing.T) {
	tests := []struct {
		name          string
		originalError error
		moduleName    string
		version       string
		operation     string
		expectType    error
		expectMessage string
	}{
		{
			name:          "Version not found error",
			originalError: errors.New("MANIFEST_UNKNOWN: manifest not found"),
			moduleName:    "test-module",
			version:       "v1.0.0",
			operation:     "get image",
			expectType:    ErrVersionNotInRegistry,
			expectMessage: "registry error for module 'test-module' version 'v1.0.0' get image: version not found in registry",
		},
		{
			name:          "404 error",
			originalError: errors.New("404 Not Found"),
			moduleName:    "test-module",
			version:       "v1.0.0",
			operation:     "get digest",
			expectType:    ErrVersionNotInRegistry,
			expectMessage: "registry error for module 'test-module' version 'v1.0.0' get digest: version not found in registry",
		},
		{
			name:          "Manifest error",
			originalError: errors.New("manifest unknown"),
			moduleName:    "test-module",
			version:       "v1.0.0",
			operation:     "load manifest",
			expectType:    errors.New("manifest unknown"),
			expectMessage: "registry error for module 'test-module' version 'v1.0.0' load manifest: manifest unknown",
		},
		{
			name:          "Other error",
			originalError: errors.New("network error"),
			moduleName:    "test-module",
			version:       "v1.0.0",
			operation:     "get digest",
			expectType:    errors.New("network error"),
			expectMessage: "registry error for module 'test-module' version 'v1.0.0' get digest: network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRegistryError(tt.originalError, tt.moduleName, tt.version, tt.operation)

			if result.Error() != tt.expectMessage {
				t.Errorf("Expected message %q, got %q", tt.expectMessage, result.Error())
			}

			var registryErr *RegistryError
			if errors.As(result, &registryErr) {
				if tt.expectType == tt.originalError {
					// For "Other error" case, we expect the original error to be wrapped
					if registryErr.Err.Error() != tt.originalError.Error() {
						t.Errorf("Expected wrapped original error %q, got %q", tt.originalError.Error(), registryErr.Err.Error())
					}
				} else if !errors.Is(registryErr.Err, tt.expectType) {
					t.Errorf("Expected wrapped error type %T, got %T", tt.expectType, registryErr.Err)
				}
			} else {
				t.Errorf("Expected RegistryError wrapper")
			}
		})
	}
}

func TestIsReleaseChannelNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Release channel not found error",
			err:      classifyReleaseChannelError(errors.New("not found"), "test", "stable", "get digest"),
			expected: true,
		},
		{
			name:     "Registry error",
			err:      classifyRegistryError(errors.New("not found"), "test", "v1.0.0", "get digest"),
			expected: false,
		},
		{
			name:     "Regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReleaseChannelNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsVersionNotInRegistryError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Version not in registry error",
			err:      classifyRegistryError(errors.New("not found"), "test", "v1.0.0", "get digest"),
			expected: true,
		},
		{
			name:     "Release channel error",
			err:      classifyReleaseChannelError(errors.New("not found"), "test", "stable", "get digest"),
			expected: false,
		},
		{
			name:     "Regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVersionNotInRegistryError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
