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

package dto

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
)

const (
	// DefinitionFile is the filename for package metadata
	DefinitionFile = "package.yaml"
)

// Definition represents package metadata loaded from package.yaml.
// It contains package identification, descriptions, requirements, and configuration options.
type Definition struct {
	Name    string `yaml:"name" json:"name"`
	Type    string `yaml:"type" json:"type"`
	Version string `yaml:"version" json:"version"`
	Stage   string `yaml:"stage" json:"stage"`

	Descriptions   Descriptions   `json:"descriptions,omitempty" yaml:"descriptions,omitempty"`
	Requirements   Requirements   `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	DisableOptions DisableOptions `json:"disable,omitempty" yaml:"disable,omitempty"`
}

// Descriptions holds localized description text for the package.
type Descriptions struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

// Requirements specifies dependencies required by this package.
type Requirements struct {
	Kubernetes string            `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	Deckhouse  string            `yaml:"deckhouse,omitempty" json:"deckhouse,omitempty"`
	Modules    map[string]string `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// DisableOptions configures package disablement behavior.
type DisableOptions struct {
	Confirmation bool   `json:"confirmation" yaml:"confirmation"` // Whether confirmation is required to disable
	Message      string `json:"message" yaml:"message"`           // Message to display when disabling
}

// ToApplication converts package definition to application definition
func (d *Definition) ToApplication() (apps.Definition, error) {
	var err error

	var kubernetesConstraint *semver.Constraints
	if len(d.Requirements.Kubernetes) > 0 {
		if kubernetesConstraint, err = semver.NewConstraint(d.Requirements.Kubernetes); err != nil {
			return apps.Definition{}, fmt.Errorf("parse kubernetes requirement: %w", err)
		}
	}

	var deckhouseConstraint *semver.Constraints
	if len(d.Requirements.Deckhouse) > 0 {
		if deckhouseConstraint, err = semver.NewConstraint(d.Requirements.Deckhouse); err != nil {
			return apps.Definition{}, fmt.Errorf("parse deckhouse requirement: %w", err)
		}
	}

	modules := make(map[string]apps.Dependency)
	for module, rawConstraint := range d.Requirements.Modules {
		raw, optional := strings.CutSuffix(rawConstraint, "!optional")
		constraint, err := semver.NewConstraint(raw)
		if err != nil {
			return apps.Definition{}, fmt.Errorf("parse module requirement '%s': %w", module, err)
		}

		modules[module] = apps.Dependency{
			Constraints: constraint,
			Optional:    optional,
		}
	}

	return apps.Definition{
		Name:    d.Name,
		Version: d.Version,
		Stage:   d.Stage,
		DisableOptions: apps.DisableOptions{
			Confirmation: d.DisableOptions.Confirmation,
			Message:      d.DisableOptions.Message,
		},
		Requirements: apps.Requirements{
			Kubernetes: kubernetesConstraint,
			Deckhouse:  deckhouseConstraint,
			Modules:    modules,
		},
	}, nil
}
