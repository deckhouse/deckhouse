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

package types // nolint:revive

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
  deckhouse: ">= 1.67"
  kubernetes: ">= 1.31"
  modules:
    prometheus: ">= 0.0.0"
    control-plane-manager: ">= 0.0.0"
accessibility:
  editions:
    ee:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
`

	var m Definition

	err := yaml.Unmarshal([]byte(data), &m)
	require.NoErrorf(t, err, "try unmarshal yaml\n%s", data)

	assert.Equal(t, "testxxz", m.Name)
	assert.Equal(t, uint32(340), m.Weight)
	assert.Equal(t, ">= 1.31", m.Requirements.Kubernetes)
	assert.Equal(t, ">= 1.67", m.Requirements.Deckhouse)
	assert.Equal(t, ">= 0.0.0", m.Requirements.ParentModules["prometheus"])
	assert.Equal(t, false, m.Critical)

	// check accessibility is parsed successfully
	assert.NotNil(t, m.Accessibility)
	// check edition is parsed successfully
	assert.NotEmpty(t, m.Accessibility.Editions)
	// check ee edition is parsed successfully
	assert.NotEmpty(t, m.Accessibility.Editions["ee"])
	// check bundles are parsed successfully
	assert.NotEmpty(t, m.Accessibility.Editions["ee"].EnabledInBundles)
	// check module is unavailable in se
	assert.False(t, m.Accessibility.IsAvailable("se"))
	// check module is available in ee
	assert.True(t, m.Accessibility.IsAvailable("ee"))
	// check module is enabled in ee/Minimal
	assert.True(t, m.Accessibility.IsEnabled("ee", "Minimal"))
	// check module is not enabled in ee/Default
	assert.False(t, m.Accessibility.IsEnabled("ee", "Default"))
	// check module is not enabled in ee/Minimal
	assert.False(t, m.Accessibility.IsEnabled("be", "Minimal"))
	// check mapping to v1alpha1
	assert.NotEmpty(t, m.Accessibility.ToV1Alpha1())
}

func TestModuleAccessibilityIsEnabled(t *testing.T) {
	// Regression from #20958: an availability-only edition entry (no enabledInBundles)
	// must inherit the _default bundle enablement instead of silently disabling the module.
	availabilityOnly := &ModuleAccessibility{
		Editions: map[string]ModuleEdition{
			"_default": {Available: true, EnabledInBundles: []string{"Default", "Managed"}},
			"cse":      {Available: true}, // no enabledInBundles -> inherit _default
		},
	}
	assert.True(t, availabilityOnly.IsEnabled("cse", "Default"), "availability-only cse entry must inherit _default bundles")
	assert.True(t, availabilityOnly.IsEnabled("cse", "Managed"))
	assert.False(t, availabilityOnly.IsEnabled("cse", "Minimal"), "Minimal is not in _default bundles")
	assert.True(t, availabilityOnly.IsAvailable("cse"))

	// An edition entry that declares its own bundles stays authoritative: it does NOT
	// merge with _default, so a bundle only present in _default is not enabled for it.
	authoritative := &ModuleAccessibility{
		Editions: map[string]ModuleEdition{
			"_default": {Available: true, EnabledInBundles: []string{"Default"}},
			"ee":       {Available: true, EnabledInBundles: []string{"Minimal"}},
		},
	}
	assert.True(t, authoritative.IsEnabled("ee", "Minimal"))
	assert.False(t, authoritative.IsEnabled("ee", "Default"), "ee declares its own bundles and must not inherit _default's Default")
	// an edition without its own entry falls back to _default
	assert.True(t, authoritative.IsEnabled("ce", "Default"))
	assert.False(t, authoritative.IsEnabled("ce", "Minimal"))

	// no _default and no matching edition entry -> not enabled
	noDefault := &ModuleAccessibility{
		Editions: map[string]ModuleEdition{
			"ee": {Available: true, EnabledInBundles: []string{"Default"}},
		},
	}
	assert.False(t, noDefault.IsEnabled("cse", "Default"))
	assert.True(t, noDefault.IsEnabled("ee", "Default"))
}
