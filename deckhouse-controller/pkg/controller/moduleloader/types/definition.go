// Copyright 2024 Flant JSC
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
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	DefinitionFile = "module.yaml"

	ExperimentalModuleStage = "Experimental"
	DeprecatedModuleStage   = "Deprecated"
)

// Definition of module.yaml file struct
type Definition struct {
	Name           string   `json:"name" yaml:"name"`
	Critical       bool     `json:"critical,omitempty" yaml:"critical,omitempty"`
	Weight         uint32   `json:"weight,omitempty" yaml:"weight,omitempty"`
	Tags           []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Subsystems     []string `json:"subsystems,omitempty" yaml:"subsystems,omitempty"`
	Namespace      string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Stage          string   `json:"stage,omitempty" yaml:"stage,omitempty"`
	ExclusiveGroup string   `json:"exclusiveGroup,omitempty" yaml:"exclusiveGroup,omitempty"`

	Descriptions  *ModuleDescriptions          `json:"descriptions,omitempty" yaml:"descriptions,omitempty"`
	Requirements  *v1alpha1.ModuleRequirements `json:"requirements,omitempty" yaml:"requirements,omitempty"`
	Accessibility *ModuleAccessibility         `json:"accessibility,omitempty" yaml:"accessibility,omitempty"`

	DisableOptions *v1alpha1.ModuleDisableOptions `json:"disable,omitempty" yaml:"disable,omitempty"`
	Path           string                         `json:"-" yaml:"-"`

	// Update holds version transition hints that allow skipping step-by-step upgrades.
	// Example:
	// update:
	//   versions:
	//     - from: 1.67
	//       to: 1.75
	//     - from: 1.20
	//       to: 2.0
	Update *ModuleUpdate `json:"update,omitempty" yaml:"update,omitempty"`
}

// ModuleUpdate describes allowed version transitions for a target release version.
type ModuleUpdate struct {
	Versions []ModuleUpdateVersion `json:"versions,omitempty" yaml:"versions,omitempty"`
}

func (a *ModuleUpdate) ToV1Alpha1() *v1alpha1.UpdateSpec {
	if a == nil {
		return nil
	}

	us := new(v1alpha1.UpdateSpec)

	us.Versions = make([]v1alpha1.UpdateConstraint, 0, len(a.Versions))

	for _, ver := range a.Versions {
		us.Versions = append(us.Versions, v1alpha1.UpdateConstraint{
			From: ver.From,
			To:   ver.To,
		})
	}

	return us
}

// ModuleUpdateVersion represents a constraint range.
// "from" and "to" support major.minor or major.minor.patch.
// "to" should point to the target release version defined by this module.yaml.
type ModuleUpdateVersion struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

type ModuleAccessibility struct {
	Editions map[string]ModuleEdition `json:"editions" yaml:"editions"`
}

type ModuleEdition struct {
	Available        bool     `json:"available" yaml:"available"`
	EnabledInBundles []string `json:"enabledInBundles" yaml:"enabledInBundles"`
}

// IsAvailable checks if the module available in the specific edition
func (a *ModuleAccessibility) IsAvailable(editionName string) bool {
	if a == nil {
		return false
	}

	if len(a.Editions) == 0 {
		return false
	}

	// edition‑specific lookup, falling back to the default settings
	if edition, ok := a.Editions[editionName]; ok {
		return edition.Available
	}

	// check the default settings
	defaultSettings, ok := a.Editions["_default"]
	if !ok {
		return false
	}

	// fallback to the default
	return defaultSettings.Available
}

// IsEnabled checks if the module enabled in the specific edition and bundle
func (a *ModuleAccessibility) IsEnabled(editionName, bundleName string) bool {
	if a == nil {
		return false
	}

	if len(a.Editions) == 0 {
		return false
	}

	// check edition‑specific bundles first
	if edition, ok := a.Editions[editionName]; ok && isEnabledInBundle(edition.EnabledInBundles, bundleName) {
		return true
	}

	// check the default settings
	defaultSettings, ok := a.Editions["_default"]
	if !ok {
		return false
	}

	// fallback to the default
	return isEnabledInBundle(defaultSettings.EnabledInBundles, bundleName)
}

func isEnabledInBundle(bundles []string, requested string) bool {
	for _, bundle := range bundles {
		if bundle == requested {
			return true
		}
	}

	return false
}

func (a *ModuleAccessibility) ToV1Alpha1() *v1alpha1.ModuleAccessibility {
	if a == nil {
		return nil
	}

	accessCopy := new(v1alpha1.ModuleAccessibility)

	accessCopy.Editions = make(map[string]v1alpha1.ModuleEdition, len(a.Editions))

	for name, edition := range a.Editions {
		accessCopy.Editions[name] = v1alpha1.ModuleEdition{
			Available:        edition.Available,
			EnabledInBundles: slices.Clone(edition.EnabledInBundles),
		}
	}

	return accessCopy
}

type ModuleDescriptions struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

func (d *Definition) Validate(values addonutils.Values, logger *log.Logger) error {
	if d.Path == "" {
		return errors.New("cannot validate module without path. Path is required to load openapi specs")
	}

	cb, vb, err := addonutils.ReadOpenAPIFiles(filepath.Join(d.Path, "openapi"))
	if err != nil {
		return fmt.Errorf("read open API files: %w", err)
	}

	dm, err := addonmodules.NewBasicModule(d.Name, d.Path, d.Weight, nil, cb, vb, addonmodules.WithLogger(logger.Named("basic-module")))
	if err != nil {
		return fmt.Errorf("new basic module: %w", err)
	}

	if values != nil {
		dm.SaveConfigValues(values)
	}

	err = dm.Validate()

	// next we will need to record all validation errors
	var result error
	var mErr *multierror.Error
	if errors.As(err, &mErr) {
		for _, me := range mErr.Errors {
			result = errors.Join(result, me)
		}
	}

	// now result will contain all validation errors
	if result != nil {
		return fmt.Errorf("validate module: %w", result)
	}

	return nil
}

func (d *Definition) Annotations() map[string]string {
	annotations := make(map[string]string)

	if d.Descriptions != nil {
		if len(d.Descriptions.Ru) > 0 {
			annotations[v1alpha1.ModuleAnnotationDescriptionRu] = d.Descriptions.Ru
		}
		if len(d.Descriptions.En) > 0 {
			annotations[v1alpha1.ModuleAnnotationDescriptionEn] = d.Descriptions.En
		}
	}

	return annotations
}

func (d *Definition) Labels() map[string]string {
	labels := make(map[string]string)

	if strings.HasPrefix(d.Name, "cni-") {
		labels["module.deckhouse.io/cni"] = ""
	}

	if strings.HasPrefix(d.Name, "cloud-provider-") {
		labels["module.deckhouse.io/cloud-provider"] = ""
	}

	if len(d.Tags) != 0 {
		for _, tag := range d.Tags {
			labels["module.deckhouse.io/"+tag] = ""
		}
	}

	return labels
}

func (d *Definition) IsExperimental() bool {
	return d.Stage == ExperimentalModuleStage
}

func (d *Definition) IsDeprecated() bool {
	return d.Stage == DeprecatedModuleStage
}
