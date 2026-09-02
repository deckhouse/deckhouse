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

package pkgsync

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestConfiguredSource(t *testing.T) {
	assert.Empty(t, ConfiguredSource(nil))
	assert.Empty(t, ConfiguredSource(&v1alpha1.ModuleConfig{}))
	assert.Empty(t, ConfiguredSource(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: v1alpha1.ModuleSourceEmbedded}}))
	assert.Equal(t, "mirror", ConfiguredSource(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: "mirror"}}))
}

func TestCatalogRepository(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		offering   []string
		want       string
	}{
		{name: "nobody offers", want: ""},
		{name: "single source", offering: []string{"mirror"}, want: "mirror"},
		{name: "single deckhouse source maps to its repository", offering: []string{"deckhouse"}, want: "deckhouse-modules"},
		{name: "several sources and no choice", offering: []string{"deckhouse", "mirror"}, want: ""},
		{name: "configured source wins", configured: "mirror", offering: []string{"deckhouse", "mirror"}, want: "mirror"},
		{name: "configured deckhouse source maps to its repository", configured: "deckhouse", offering: []string{"deckhouse", "mirror"}, want: "deckhouse-modules"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CatalogRepository(tt.configured, tt.offering))
		})
	}
}

func TestCatalogConflict(t *testing.T) {
	assert.False(t, CatalogConflict(false, "", []string{"deckhouse", "mirror"}), "a disabled module is never in conflict")
	assert.False(t, CatalogConflict(true, "mirror", []string{"deckhouse", "mirror"}), "a chosen source settles the conflict")
	assert.False(t, CatalogConflict(true, "", []string{"mirror"}), "a single source is no conflict")
	assert.True(t, CatalogConflict(true, "", []string{"deckhouse", "mirror"}))
}
