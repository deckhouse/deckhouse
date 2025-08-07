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

package checker

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
)

func TestUpdateImageRepo(t *testing.T) {
	tests := []struct {
		name           string
		imageRef       string
		newRepo        string
		expectError    bool
		expectedNewRef string
	}{
		{
			name:           "Valid Tag Reference",
			imageRef:       "gcr.io/a/b/c/d/e:latest",
			newRepo:        "gcr.io/new/repo",
			expectError:    false,
			expectedNewRef: "gcr.io/new/repo:latest",
		},
		{
			name:           "Valid Digest Reference",
			imageRef:       "gcr.io/a/b/c/d/e@sha256:3e23e8160039594a33894f6564e1b1348bb931caaa48e487d6a6e3c7d4f65975",
			newRepo:        "gcr.io/new/repo",
			expectError:    false,
			expectedNewRef: "gcr.io/new/repo@sha256:3e23e8160039594a33894f6564e1b1348bb931caaa48e487d6a6e3c7d4f65975",
		},
		{
			name:        "Invalid Reference",
			imageRef:    "invalid-image-ref",
			newRepo:     "gcr.io/new/repo",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRepo, err := name.NewRepository(tt.newRepo)
			assert.NoError(t, err, "failed to create new repository")

			newRef, err := updateImageRepo(tt.imageRef, newRepo)

			if tt.expectError {
				assert.Error(t, err, "expected error during image repo update")
			} else {
				assert.NoError(t, err, "unexpected error during image repo update")
				assert.Equal(t, tt.expectedNewRef, newRef.String())
			}
		})
	}
}
