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

package definition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
)

// moduleYAML exercises every field of the legacy manifest, including the
// requirement shape that is unique to it: bare version ranges and a flat
// module map.
const moduleYAML = `
name: stronghold
critical: true
weight: 910
tags:
  - security
subsystems:
  - secrets
namespace: d8-stronghold
stage: General Availability
exclusiveGroup: vault
descriptions:
  ru: Хранилище секретов
  en: Secrets storage
requirements:
  deckhouse: ">= 1.70"
  kubernetes: ">= 1.27"
  bootstrapped: "true"
  modules:
    user-authn: ">= 1.0.0"
accessibility:
  editions:
    _default:
      available: false
      enabledInBundles: []
    ee:
      available: true
      enabledInBundles: ["Default", "Managed"]
disable:
  confirmation: true
  message: legacy text
  messages:
    ru: Точно?
    en: Are you sure?
update:
  versions:
    - from: "1.67"
      to: "1.75"
`

// packageYAML exercises the v2 manifest, whose requirements wrap each
// constraint in an object and split module dependencies into buckets.
const packageYAML = `
apiVersion: deckhouse.io/v1alpha1
type: Module
name: stronghold
version: v1.0.1
stage: General Availability
weight: 910
critical: true
descriptions:
  ru: Хранилище секретов
  en: Secrets storage
requirements:
  kubernetes:
    constraint: ">= 1.27"
  deckhouse:
    constraint: ">= 1.70"
  modules:
    mandatory:
      - name: user-authn
        constraint: ">= 1.0.0"
    conditional:
      - name: prometheus
        constraint: ">= 2.0.0"
    anyOf:
      - name: cni
        description: any CNI
        modules:
          - name: cni-cilium
          - name: cni-flannel
    noneOf:
      - name: conflicting
        modules:
          - name: vault-legacy
licensing:
  editions:
    ee:
      available: true
      enabledInBundles: ["Default"]
disable:
  confirmation: true
  messages:
    ru: Точно?
    en: Are you sure?
`

func TestParseModule(t *testing.T) {
	got, err := definition.ParseModule([]byte(moduleYAML))
	require.NoError(t, err)

	assert.Equal(t, "stronghold", got.Name)
	assert.True(t, got.Critical)
	assert.Equal(t, uint32(910), got.Weight)
	assert.Equal(t, []string{"security"}, got.Tags)
	assert.Equal(t, []string{"secrets"}, got.Subsystems)
	assert.Equal(t, "d8-stronghold", got.Namespace)
	assert.Equal(t, "General Availability", got.Stage)
	assert.Equal(t, "vault", got.ExclusiveGroup)

	require.NotNil(t, got.Descriptions)
	assert.Equal(t, "Secrets storage", got.Descriptions.En)

	// module.yaml states requirements as bare ranges plus a flat module map.
	require.NotNil(t, got.Requirements)
	assert.Equal(t, ">= 1.70", got.Requirements.Deckhouse)
	assert.Equal(t, ">= 1.27", got.Requirements.Kubernetes)
	assert.Equal(t, "true", got.Requirements.Bootstrapped)
	assert.Equal(t, map[string]string{"user-authn": ">= 1.0.0"}, got.Requirements.ParentModules)

	require.NotNil(t, got.Accessibility)
	assert.False(t, got.Accessibility.Editions["_default"].Available)
	assert.True(t, got.Accessibility.Editions["ee"].Available)
	assert.Equal(t, []string{"Default", "Managed"}, got.Accessibility.Editions["ee"].EnabledInBundles)

	require.NotNil(t, got.DisableOptions)
	assert.True(t, got.DisableOptions.Confirmation)
	assert.Equal(t, "legacy text", got.DisableOptions.Message)
	assert.Equal(t, "Are you sure?", got.DisableOptions.Messages.En)

	require.NotNil(t, got.Update)
	require.Len(t, got.Update.Versions, 1)
	assert.Equal(t, "1.67", got.Update.Versions[0].From)
	assert.Equal(t, "1.75", got.Update.Versions[0].To)
}

func TestParsePackage(t *testing.T) {
	got, err := definition.ParsePackage([]byte(packageYAML))
	require.NoError(t, err)

	assert.Equal(t, "deckhouse.io/v1alpha1", got.APIVersion)
	assert.Equal(t, definition.TypeModule, got.Type)
	assert.True(t, got.IsModule())
	assert.False(t, got.IsApplication())

	assert.Equal(t, "stronghold", got.Name)
	assert.Equal(t, "v1.0.1", got.Version)
	assert.Equal(t, "General Availability", got.Stage)
	assert.Equal(t, 910, got.Weight)
	assert.True(t, got.Critical)
	assert.Equal(t, "Secrets storage", got.Descriptions.En)

	// package.yaml wraps each platform constraint in an object.
	assert.Equal(t, ">= 1.27", got.Requirements.Kubernetes.Constraint)
	assert.Equal(t, ">= 1.70", got.Requirements.Deckhouse.Constraint)

	mods := got.Requirements.Modules
	require.Len(t, mods.Mandatory, 1)
	assert.Equal(t, "user-authn", mods.Mandatory[0].Name)
	assert.Equal(t, ">= 1.0.0", mods.Mandatory[0].Constraint)

	require.Len(t, mods.Conditional, 1)
	assert.Equal(t, "prometheus", mods.Conditional[0].Name)

	require.Len(t, mods.AnyOf, 1)
	assert.Equal(t, "cni", mods.AnyOf[0].Name)
	assert.Equal(t, "any CNI", mods.AnyOf[0].Description)
	require.Len(t, mods.AnyOf[0].Modules, 2)
	assert.Equal(t, "cni-cilium", mods.AnyOf[0].Modules[0].Name)
	assert.Empty(t, mods.AnyOf[0].Modules[0].Constraint)

	require.Len(t, mods.NoneOf, 1)
	assert.Equal(t, "vault-legacy", mods.NoneOf[0].Modules[0].Name)

	assert.True(t, got.Licensing.Editions["ee"].Available)
	assert.Equal(t, []string{"Default"}, got.Licensing.Editions["ee"].EnabledInBundles)

	assert.True(t, got.DisableOptions.Confirmation)
	assert.Equal(t, "Точно?", got.DisableOptions.Messages.Ru)
}

// TestParsePackageApplication covers the other package type sharing the schema.
func TestParsePackageApplication(t *testing.T) {
	got, err := definition.ParsePackage([]byte("type: Application\nname: elma\nversion: v1.0.1\n"))
	require.NoError(t, err)

	assert.True(t, got.IsApplication())
	assert.False(t, got.IsModule())
	assert.Equal(t, "elma", got.Name)

	// Weight and Critical are module-only and stay zero here.
	assert.Zero(t, got.Weight)
	assert.False(t, got.Critical)
}

// TestParseMinimal covers a manifest with only the required field: optional
// pointers stay nil rather than materializing empty structs.
func TestParseMinimal(t *testing.T) {
	module, err := definition.ParseModule([]byte("name: stronghold\n"))
	require.NoError(t, err)

	assert.Equal(t, "stronghold", module.Name)
	assert.Nil(t, module.Requirements)
	assert.Nil(t, module.Accessibility)
	assert.Nil(t, module.DisableOptions)
	assert.Nil(t, module.Update)
	assert.Nil(t, module.Descriptions)
}

func TestParseInvalid(t *testing.T) {
	_, err := definition.ParseModule([]byte("\tname: [unclosed"))
	require.Error(t, err)

	_, err = definition.ParsePackage([]byte("\tname: [unclosed"))
	require.Error(t, err)
}
