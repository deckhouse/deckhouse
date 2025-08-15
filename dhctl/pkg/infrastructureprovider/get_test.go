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

package infrastructureprovider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestGetCloudsUseOpentofu(t *testing.T) {
	m, err := getCloudNameToUseOpentofuMap(config.InfrastructureVersions)
	require.NoError(t, err)

	require.Len(t, m, 5)
	require.Contains(t, m, "yandex")
	require.Contains(t, m, "dynamix")
	require.Contains(t, m, "zvirt")
	require.Contains(t, m, "dvp")
}

func TestNeedToUseOpentofu(t *testing.T) {
	metaConfig := &config.MetaConfig{}

	metaConfig.ProviderName = "Yandex"
	require.True(t, NeedToUseOpentofu(metaConfig))

	notTofuProviders := []string{
		"OpenStack",
		"AWS",
		"GCP",
		"vSphere",
		"Azure",
		"VCD",
		"Huaweicloud",
	}

	for _, provider := range notTofuProviders {
		conf := &config.MetaConfig{}
		conf.ProviderName = provider

		require.False(t, NeedToUseOpentofu(conf))
	}
}
