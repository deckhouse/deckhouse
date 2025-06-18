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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshal(t *testing.T) {
	data := `
name: testxxz
weight: 340

requirements:
  bootstrapped: true
  deckhouse: ">= 1.67"
  kubernetes: ">= 1.31"
  modules:
    prometheus: ">= 0.0.0"
    control-plane-manager: ">= 0.0.0"
accessibility:
  batches:
    ee-networking:
      available: true
      enabled: true
      featureFlags: ["a", "b", "c"]

  editions:
    _default:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
    ee:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
      featureFlags: ["AllowSomeSuperFeature"]
    se:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
      featureFlags: ["SomeLimit:50"]
    be:
      available: false
`

	var m Definition

	err := yaml.Unmarshal([]byte(data), &m)
	require.NoErrorf(t, err, "try unmarshal yaml\n%s", data)

	// assert.True(t, m.Requirements.Bootstrapped)
	assert.Equal(t, "testxxz", m.Name)
	assert.Equal(t, uint32(340), m.Weight)
	assert.Equal(t, "true", m.Requirements.Bootstrapped)
	assert.Equal(t, ">= 1.31", m.Requirements.Kubernetes)
	assert.Equal(t, ">= 1.67", m.Requirements.Deckhouse)
	assert.Equal(t, ">= 0.0.0", m.Requirements.ParentModules["prometheus"])
	assert.NotNil(t, m.Accessibility)
	assert.NotEmpty(t, m.Accessibility.Editions)
	assert.True(t, m.Accessibility.Editions.Default.Available)
	assert.Contains(t, m.Accessibility.Editions.Default.EnabledInBundles, Bundle("Minimal"))
	assert.Contains(t, m.Accessibility.Editions.Default.EnabledInBundles, Bundle("Managed"))
	assert.Contains(t, m.Accessibility.Editions.Default.EnabledInBundles, Bundle("Default"))
	assert.Empty(t, m.Accessibility.Editions.Default.FeatureFlags)
	assert.True(t, m.Accessibility.Editions.Ee.Available)
	assert.Contains(t, m.Accessibility.Editions.Ee.EnabledInBundles, Bundle("Minimal"))
	assert.Contains(t, m.Accessibility.Editions.Ee.EnabledInBundles, Bundle("Managed"))
	assert.Contains(t, m.Accessibility.Editions.Ee.EnabledInBundles, Bundle("Default"))
	assert.Contains(t, m.Accessibility.Editions.Ee.FeatureFlags, FeatureFlag("AllowSomeSuperFeature"))
	assert.True(t, m.Accessibility.Editions.Se.Available)
	assert.Contains(t, m.Accessibility.Editions.Se.EnabledInBundles, Bundle("Minimal"))
	assert.Contains(t, m.Accessibility.Editions.Se.EnabledInBundles, Bundle("Managed"))
	assert.Contains(t, m.Accessibility.Editions.Se.EnabledInBundles, Bundle("Default"))
	assert.Contains(t, m.Accessibility.Editions.Se.FeatureFlags, FeatureFlag("SomeLimit:50"))
	assert.False(t, m.Accessibility.Editions.Be.Available)
	assert.Empty(t, m.Accessibility.Editions.Be.EnabledInBundles)
	assert.Empty(t, m.Accessibility.Editions.Be.FeatureFlags)
	assert.NotEmpty(t, m.Accessibility.Batches)
}
