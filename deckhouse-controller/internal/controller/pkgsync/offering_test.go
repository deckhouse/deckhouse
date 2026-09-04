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

func TestConfiguredRepository(t *testing.T) {
	assert.Empty(t, ConfiguredRepository(nil))
	assert.Empty(t, ConfiguredRepository(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: v1alpha1.ModuleSourceEmbedded}}))
	assert.Equal(t, "mirror", ConfiguredRepository(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: "mirror"}}))
	assert.Equal(t, "deckhouse-modules", ConfiguredRepository(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: "deckhouse"}}), "the platform source names its modules repository")
}

func TestPickRepository(t *testing.T) {
	tests := []struct {
		name         string
		configured   string
		repositories []string
		want         string
	}{
		{name: "nobody offers", want: ""},
		{name: "single repository", repositories: []string{"mirror"}, want: "mirror"},
		{name: "several repositories and no choice", repositories: []string{"deckhouse-modules", "mirror"}, want: ""},
		{name: "configured repository wins", configured: "mirror", repositories: []string{"deckhouse-modules", "mirror"}, want: "mirror"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PickRepository(tt.configured, tt.repositories))
		})
	}
}

func TestHasRepositoryConflict(t *testing.T) {
	assert.False(t, HasRepositoryConflict(false, "", []string{"deckhouse-modules", "mirror"}), "a disabled module is never in conflict")
	assert.False(t, HasRepositoryConflict(true, "mirror", []string{"deckhouse-modules", "mirror"}), "a chosen repository settles the conflict")
	assert.False(t, HasRepositoryConflict(true, "", []string{"mirror"}), "a single repository is no conflict")
	assert.True(t, HasRepositoryConflict(true, "", []string{"deckhouse-modules", "mirror"}))
}
