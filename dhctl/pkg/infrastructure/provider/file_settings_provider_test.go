// Copyright 2025 Flant JSC
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

package provider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
)

var terraformProviders = []string{
	"OpenStack",
	"AWS",
	"GCP",
	"vSphere",
	"Azure",
	"VCD",
	"Huaweicloud",
}

var tofuProviders = []string{
	"Yandex",
	"Dynamix",
	"Zvirt",
	"DVP",
}

func TestAllProviderPresentInStore(t *testing.T) {
	s, err := loadTerraformVersionFileSettings(infrastructure.GetInfrastructureVersions())
	require.NoError(t, err)

	all := append(make([]string, 0), tofuProviders...)
	all = append(all, terraformProviders...)

	require.Len(t, s, len(all))
}

func TestProvidersSettings(t *testing.T) {
	s, err := loadTerraformVersionFileSettings(infrastructure.GetInfrastructureVersions())
	require.NoError(t, err)

	assertSettings := func(t *testing.T, s settingsStore, p string, assertProvider func(t *testing.T, settings Settings)) {
		require.Contains(t, s, p)
		settings := s[p]
		require.NotNil(t, settings)

		assertProvider(t, settings)

		require.NotEmpty(t, settings.CloudName())
		require.NotEmpty(t, settings.Namespace())
		require.NotEmpty(t, settings.DestinationBinary())
		require.NotEmpty(t, settings.Versions())
		require.NotEmpty(t, settings.VmResourceType())
	}

	for _, p := range tofuProviders {
		assertSettings(t, s, p, func(t *testing.T, settings Settings) {
			require.True(t, settings.UseOpenTofu())
			require.Equal(t, settings.InfrastructureVersion(), "1.9.4")
		})
	}

	for _, p := range terraformProviders {
		assertSettings(t, s, p, func(t *testing.T, settings Settings) {
			require.False(t, settings.UseOpenTofu())
			require.Equal(t, settings.InfrastructureVersion(), "0.14.8")
		})
	}
}
