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

package cr

import "testing"

func TestShouldCacheTag(t *testing.T) {
	tests := []struct {
		name        string
		registryURL string
		tag         string
		want        bool
	}{
		{
			name:        "cache versioned tag in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "v1.2.3",
			want:        true,
		},
		{
			name:        "cache stable channel in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "stable",
			want:        true,
		},
		{
			name:        "cache alpha channel in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "alpha",
			want:        true,
		},
		{
			name:        "cache beta channel in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "beta",
			want:        true,
		},
		{
			name:        "cache latest tag in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "latest",
			want:        true,
		},
		{
			name:        "cache main channel in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "main",
			want:        true,
		},
		{
			name:        "cache master channel in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "master",
			want:        true,
		},
		{
			name:        "cache any tag in release directory",
			registryURL: "registry.example.com/deckhouse/release",
			tag:         "custom-tag",
			want:        true,
		},
		{
			name:        "do not cache versioned tag outside release directory",
			registryURL: "registry.example.com/deckhouse/dev",
			tag:         "v1.2.3",
			want:        false,
		},
		{
			name:        "do not cache stable channel outside release directory",
			registryURL: "registry.example.com/deckhouse",
			tag:         "stable",
			want:        false,
		},
		{
			name:        "do not cache any tag outside release directory",
			registryURL: "registry.example.com/deckhouse",
			tag:         "latest",
			want:        false,
		},
		{
			name:        "cache in release-channel directory",
			registryURL: "registry.example.com/deckhouse/release-channel",
			tag:         "stable",
			want:        true,
		},
		{
			name:        "cache version in release-channel directory",
			registryURL: "registry.example.com/deckhouse/release-channel",
			tag:         "v1.60.0",
			want:        true,
		},
		{
			name:        "cache module versioned tag in release directory",
			registryURL: "registry.example.com/moduleName/release",
			tag:         "v1.0.0",
			want:        true,
		},
		{
			name:        "cache module stable tag in release directory",
			registryURL: "registry.example.com/moduleName/release",
			tag:         "stable",
			want:        true,
		},
		{
			name:        "do not cache module versioned tag outside release directory",
			registryURL: "registry.example.com/moduleName",
			tag:         "v1.0.0",
			want:        false,
		},
		{
			name:        "do not cache module stable tag outside release directory",
			registryURL: "registry.example.com/moduleName",
			tag:         "stable",
			want:        false,
		},
		{
			name:        "do not cache if no release in path",
			registryURL: "registry.example.com/deckhouse/staging",
			tag:         "v1.2.3",
			want:        false,
		},
		{
			name:        "cache dev image from dev-registry.deckhouse.io",
			registryURL: "dev-registry.deckhouse.io/sys/deckhouse-oss",
			tag:         "v1.49.0",
			want:        true,
		},
		{
			name:        "cache dev image with main tag",
			registryURL: "dev-registry.deckhouse.io/sys/deckhouse-oss",
			tag:         "main",
			want:        true,
		},
		{
			name:        "cache dev image with custom tag",
			registryURL: "dev-registry.deckhouse.io/deckhouse/modules",
			tag:         "feature-branch",
			want:        true,
		},
		{
			name:        "do not cache non-dev non-release registry",
			registryURL: "other-registry.example.com/deckhouse",
			tag:         "v1.0.0",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCacheTag(tt.registryURL, tt.tag)
			if got != tt.want {
				t.Errorf("shouldCacheTag(%q, %q) = %v, want %v", tt.registryURL, tt.tag, got, tt.want)
			}
		})
	}
}
