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

package main

import "testing"

// ---------------------------------------------------------------------------
// parsePackageWithDigest
// ---------------------------------------------------------------------------

func TestParsePackageWithDigest(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPkg    string
		wantDigest string
		wantErr    bool
	}{
		{
			name:       "valid sha256",
			input:      "containerd:sha256:abc123",
			wantPkg:    "containerd",
			wantDigest: "sha256:abc123",
		},
		{
			name:    "missing digest part",
			input:   "containerd",
			wantErr: true,
		},
		{
			name:    "empty package name",
			input:   ":sha256:abc123",
			wantErr: true,
		},
		{
			name:    "digest without algorithm",
			input:   "containerd:abc123",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, digest, err := parsePackageWithDigest(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePackageWithDigest(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePackageWithDigest(%q) error = %v", tt.input, err)
			}
			if pkg != tt.wantPkg {
				t.Errorf("parsePackageWithDigest(%q) pkg = %q, want %q", tt.input, pkg, tt.wantPkg)
			}
			if digest != tt.wantDigest {
				t.Errorf("parsePackageWithDigest(%q) digest = %q, want %q", tt.input, digest, tt.wantDigest)
			}
		})
	}
}
