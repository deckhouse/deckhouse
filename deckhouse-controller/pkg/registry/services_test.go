// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestDeckhouseRegistry covers the releases side: the registry secret's address
// is the edition root, so the release channel hangs off it unchanged.
func TestDeckhouseRegistry(t *testing.T) {
	reg, err := deckhouseRegistry("registry.example.com/deckhouse/fe", &utils.RegistryConfig{}, log.NewNop())
	require.NoError(t, err)

	assert.Equal(t, "registry.example.com/deckhouse/fe", reg.Deckhouse().Path())
	assert.Equal(t, "registry.example.com/deckhouse/fe/release-channel", reg.Deckhouse().Releases().Path())
}

// TestModuleCatalog covers the module side: a ModuleSource repo is the catalog
// itself and is addressed as-is, whatever its path. No /modules is appended and
// nothing is trimmed, so catalogs named differently work too.
func TestModuleCatalog(t *testing.T) {
	tests := []struct {
		repo   string
		module string
	}{
		{"registry.example.com/deckhouse/fe/modules", "registry.example.com/deckhouse/fe/modules"},
		{"registry.example.io/external-modules", "registry.example.io/external-modules"},
		{"registry.example.io/modules-source", "registry.example.io/modules-source"},
	}

	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			catalog, err := moduleCatalog(tt.repo, &utils.RegistryConfig{}, log.NewNop())
			require.NoError(t, err)

			assert.Equal(t, tt.repo, catalog.Path())
			assert.Equal(t, tt.module+"/stronghold", catalog.Module("stronghold").Path())
			assert.Equal(t, tt.module+"/stronghold/release", catalog.Module("stronghold").Releases().Path())
		})
	}
}

func TestDeckhouseRegistryBadDockerConfig(t *testing.T) {
	_, err := deckhouseRegistry(
		"registry.example.com/deckhouse/fe",
		&utils.RegistryConfig{DockerConfig: "%%% not base64 %%%"},
		log.NewNop(),
	)
	require.Error(t, err)
}

func TestModuleCatalogBadDockerConfig(t *testing.T) {
	_, err := moduleCatalog(
		"registry.example.io/modules",
		&utils.RegistryConfig{DockerConfig: "%%% not base64 %%%"},
		log.NewNop(),
	)
	require.Error(t, err)
}
