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

package apps

import (
	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
)

// Definition represents application metadata.
type Definition struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Stage   string `json:"stage" yaml:"stage"`

	Requirements   Requirements   `json:"requirements" yaml:"requirements"`
	DisableOptions DisableOptions `json:"disableOptions" yaml:"disableOptions"`
}

// Requirements specifies dependencies required by the application.
type Requirements struct {
	Kubernetes *semver.Constraints   `json:"kubernetes" yaml:"kubernetes"`
	Deckhouse  *semver.Constraints   `json:"deckhouse" yaml:"deckhouse"`
	Modules    map[string]Dependency `json:"modules" yaml:"modules"`
}

type Dependency struct {
	Constraints *semver.Constraints `json:"constraints" yaml:"constraints"`
	Optional    bool                `json:"optional" yaml:"optional"`
}

// DisableOptions configures application disablement behavior.
type DisableOptions struct {
	Confirmation bool   `json:"confirmation" yaml:"confirmation"` // Whether confirmation is required to disable
	Message      string `json:"message" yaml:"message"`           // Message to display when disabling
}

func (r *Requirements) Checks() schedule.Checks {
	if r == nil {
		return schedule.Checks{}
	}

	deps := make(map[string]dependency.Dependency)
	for module, dep := range r.Modules {
		deps[module] = dependency.Dependency{
			Constraint: dep.Constraints,
			Optional:   dep.Optional,
		}
	}

	return schedule.Checks{
		Kubernetes: r.Kubernetes,
		Deckhouse:  r.Deckhouse,
		Modules:    deps,
	}
}
