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

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestFromPackageDefinition(t *testing.T) {
	t.Run("maps every carried field", func(t *testing.T) {
		pd := &dto.ModuleDefinition{
			Definition: dto.Definition{
				Stage:        "General Availability",
				Descriptions: dto.Descriptions{Ru: "ru description", En: "en description"},
				DisableOptions: dto.DisableOptions{
					Confirmation: true,
					Messages:     dto.DisableMessages{Ru: "ru do not", En: "en do not"},
				},
				Licensing: dto.Licensing{
					Editions: map[string]dto.Edition{
						"ee": {Available: true, EnabledInBundles: []string{"Default"}},
					},
				},
				Requirements: dto.Requirements{
					Kubernetes: dto.VersionConstraint{Constraint: ">= 1.27"},
					Deckhouse:  dto.VersionConstraint{Constraint: ">= 1.70"},
					Modules: dto.ModulesRequirements{
						Mandatory:   []dto.ModuleDependency{{Name: "cert-manager", Constraint: ">= 1.0.0"}},
						Conditional: []dto.ModuleDependency{{Name: "prometheus"}},
						AnyOf: []dto.ModuleGroup{{
							Name:        "cni",
							Description: "one CNI must be present",
							Modules:     []dto.ModuleDependency{{Name: "cni-cilium"}, {Name: "cni-flannel"}},
						}},
					},
				},
			},
			Weight:         910,
			Critical:       true,
			ExclusiveGroup: "ingress",
		}

		meta := FromPackageDefinition(pd)

		assert.Equal(t, "General Availability", meta.Stage)
		assert.Equal(t, "ru description", meta.Description.Ru)
		assert.Equal(t, "en description", meta.Description.En)
		assert.Equal(t, int32(910), meta.Weight)
		assert.True(t, meta.Critical)
		assert.Equal(t, "ingress", meta.ExclusiveGroup)

		require.NotNil(t, meta.DisableOptions)
		assert.True(t, meta.DisableOptions.Confirmation)
		assert.Equal(t, "en do not", meta.DisableOptions.Messages.En)

		require.NotNil(t, meta.Licensing)
		assert.True(t, meta.Licensing.Editions["ee"].Available)
		assert.Equal(t, []string{"Default"}, meta.Licensing.Editions["ee"].EnabledInBundles)

		require.NotNil(t, meta.Requirements)
		assert.Equal(t, ">= 1.27", meta.Requirements.Kubernetes.Constraint)
		assert.Equal(t, ">= 1.70", meta.Requirements.Deckhouse.Constraint)
		require.NotNil(t, meta.Requirements.Modules)
		assert.Equal(t, "cert-manager", meta.Requirements.Modules.Mandatory[0].Name)
		assert.Equal(t, "prometheus", meta.Requirements.Modules.Conditional[0].Name)
		require.Len(t, meta.Requirements.Modules.AnyOf, 1)
		assert.Equal(t, "cni", meta.Requirements.Modules.AnyOf[0].Name)
		assert.Len(t, meta.Requirements.Modules.AnyOf[0].Modules, 2)
	})

	t.Run("empty optional sections collapse to nil", func(t *testing.T) {
		meta := FromPackageDefinition(&dto.ModuleDefinition{
			Definition: dto.Definition{Stage: "Experimental"},
		})

		assert.Nil(t, meta.DisableOptions)
		assert.Nil(t, meta.Licensing)
		assert.Nil(t, meta.Requirements)
		require.NotNil(t, meta.Description, "the description block is always present")
		assert.Empty(t, meta.Description.En)
	})
}

func TestLegacyRequirementsToCR(t *testing.T) {
	t.Run("optional suffix routes a dependency to conditional", func(t *testing.T) {
		req := LegacyRequirementsToCR(&v1alpha1.ModuleRequirements{
			ParentModules: map[string]string{
				"cert-manager": ">= 1.0.0",
				"prometheus":   ">= 2.0.0 !optional",
			},
		})

		require.NotNil(t, req)
		require.NotNil(t, req.Modules)
		require.Len(t, req.Modules.Mandatory, 1)
		assert.Equal(t, "cert-manager", req.Modules.Mandatory[0].Name)
		require.Len(t, req.Modules.Conditional, 1)
		assert.Equal(t, "prometheus", req.Modules.Conditional[0].Name)
		assert.Equal(t, ">= 2.0.0", req.Modules.Conditional[0].Constraint, "the suffix is stripped")
	})

	t.Run("no requirements collapse to nil", func(t *testing.T) {
		assert.Nil(t, LegacyRequirementsToCR(&v1alpha1.ModuleRequirements{}))
	})
}
