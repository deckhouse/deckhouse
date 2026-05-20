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

package bundle

import (
	"testing"
)

func TestLegacyPathTransform(t *testing.T) {
	tests := []struct {
		name         string
		archName     string
		layoutPath   string
		expectedPath string
	}{
		// Security archive
		{
			name:         "old security archive",
			archName:     "security.tar",
			layoutPath:   "test",
			expectedPath: "security/test",
		},
		{
			name:         "new security archive",
			archName:     "security.tar",
			layoutPath:   "security/test",
			expectedPath: "security/test",
		},

		// Module archive
		{
			name:         "old module archive",
			archName:     "module-prompp.tar",
			layoutPath:   "test",
			expectedPath: "modules/prompp/test",
		},
		{
			name:         "new module archive",
			archName:     "module-prompp.tar",
			layoutPath:   "modules/prompp/test",
			expectedPath: "modules/prompp/test",
		},

		// Other archive
		{
			name:         "platform archive",
			archName:     "platform.tar",
			layoutPath:   "some/path/a",
			expectedPath: "some/path/a",
		},
		{
			name:         "packages archive",
			archName:     "packages-prompp.tar",
			layoutPath:   "some/path/a",
			expectedPath: "some/path/a",
		},
		{
			name:         "deckhouse-cli archive",
			archName:     "deckhouse-cli.tar",
			layoutPath:   "some/path/a",
			expectedPath: "some/path/a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := legacyPathTransform(tt.archName, tt.layoutPath)
			if got != tt.expectedPath {
				t.Errorf("legacyPathTransform(%q, %q) = %q, want %q", tt.archName, tt.layoutPath, got, tt.expectedPath)
			}
		})
	}
}
