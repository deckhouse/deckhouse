// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkgsync

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestRepositoryNameForSource(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{source: "deckhouse", want: "deckhouse-modules"},
		{source: "example", want: "example"},
		{source: "deckhouse-prod", want: "deckhouse-prod"},
		{source: "", want: ""},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, RepositoryNameForSource(c.source), c.source)
	}
}

func TestSourceNameForRepository(t *testing.T) {
	cases := []struct {
		repository string
		want       string
	}{
		{repository: "deckhouse-modules", want: "deckhouse"},
		{repository: "embedded", want: ""},
		{repository: "example", want: "example"},
		{repository: "", want: ""},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, SourceNameForRepository(c.repository), c.repository)
	}
}

func TestConfiguredModuleSource(t *testing.T) {
	assert.Empty(t, ConfiguredModuleSource(nil))
	assert.Empty(t, ConfiguredModuleSource(&v1alpha1.ModuleConfig{}))
	assert.Empty(t, ConfiguredModuleSource(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: v1alpha1.ModuleSourceEmbedded}}))
	assert.Equal(t, "mirror", ConfiguredModuleSource(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: "mirror"}}))
}

func TestConfiguredRepository(t *testing.T) {
	assert.Empty(t, ConfiguredRepository(nil))
	assert.Empty(t, ConfiguredRepository(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: v1alpha1.ModuleSourceEmbedded}}))
	assert.Equal(t, "mirror", ConfiguredRepository(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: "mirror"}}))
	assert.Equal(t, "deckhouse-modules", ConfiguredRepository(&v1alpha1.ModuleConfig{Spec: v1alpha1.ModuleConfigSpec{Source: "deckhouse"}}), "the platform source names its modules repository")
}

func TestModuleSourceNamesForRepositories(t *testing.T) {
	tests := []struct {
		name         string
		repositories []string
		want         []string
	}{
		{name: "no repositories", want: []string{}},
		{name: "the platform repository names the platform module source", repositories: []string{"deckhouse-modules"}, want: []string{"deckhouse"}},
		{name: "the embedded repository names no module source", repositories: []string{"embedded"}, want: []string{}},
		{name: "sorted and without duplicates", repositories: []string{"mirror", "deckhouse-modules", "mirror", "embedded"}, want: []string{"deckhouse", "mirror"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ModuleSourceNamesForRepositories(tt.repositories))
		})
	}
}
