/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package extenders

import (
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionavailable"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionenabled"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type ExtendersStack struct {
	DeckhouseVersion  *deckhouseversion.Extender
	KubernetesVersion *kubernetesversion.Extender
	ModuleDependency  *moduledependency.Extender
	Bootstrapped      *bootstrapped.Extender
	EditionAvailable  *editionavailable.Extender
	EditionEnabled    *editionenabled.Extender
}

func NewExtendersStack(edition *d8edition.Edition, bootstrappedHelper func() (bool, error), logger *log.Logger) *ExtendersStack {
	return &ExtendersStack{
		DeckhouseVersion:  deckhouseversion.NewExtender(edition.Version, logger.Named("deckhouse-version-extender")),
		KubernetesVersion: kubernetesversion.Instance(),
		ModuleDependency:  moduledependency.Instance(),
		Bootstrapped:      bootstrapped.NewExtender(bootstrappedHelper, logger.Named("bootstrapped-extender")),
		EditionAvailable:  editionavailable.New(edition.Name, logger.Named("edition-available-extender")),
		EditionEnabled:    editionenabled.New(edition.Name, edition.Bundle, logger.Named("edition-enabled-extender")),
	}
}

func (b *ExtendersStack) GetExtenders() []extenders.Extender {
	return []extenders.Extender{
		b.DeckhouseVersion,
		b.KubernetesVersion,
		b.ModuleDependency,
		b.Bootstrapped,
		b.EditionAvailable,
		b.EditionEnabled,
	}
}

func (b *ExtendersStack) AddConstraints(module string, critical bool, access *moduletypes.ModuleAccessibility, requirements *v1alpha1.ModuleRequirements) error {
	if !critical {
		b.Bootstrapped.AddFunctionalModule(module)
	}

	if access != nil {
		b.EditionEnabled.AddModule(module, access)
		b.EditionAvailable.AddModule(module, access)
	}

	if requirements == nil {
		// no requirements
		return nil
	}

	if len(requirements.Deckhouse) > 0 {
		if err := b.DeckhouseVersion.AddConstraint(module, requirements.Deckhouse); err != nil {
			return err
		}
	}

	if len(requirements.Kubernetes) > 0 {
		if err := b.KubernetesVersion.AddConstraint(module, requirements.Kubernetes); err != nil {
			return err
		}
	}

	if len(requirements.ParentModules) > 0 {
		if err := b.ModuleDependency.AddConstraint(module, requirements.ParentModules); err != nil {
			return err
		}
	}

	return nil
}

func (b *ExtendersStack) DeleteConstraints(module string) {
	b.DeckhouseVersion.DeleteConstraint(module)
	b.KubernetesVersion.DeleteConstraint(module)
	b.ModuleDependency.DeleteConstraint(module)
}

func (b *ExtendersStack) CheckModuleReleaseRequirements(moduleName, moduleRelease string, moduleReleaseVersion *semver.Version, requirements *v1alpha1.ModuleReleaseRequirements) error {
	if requirements == nil {
		// no requirements
		return nil
	}

	if len(requirements.Deckhouse) > 0 {
		if err := b.DeckhouseVersion.ValidateRelease(moduleRelease, requirements.Deckhouse); err != nil {
			return err
		}
	}

	if len(requirements.Kubernetes) > 0 {
		if err := b.KubernetesVersion.ValidateRelease(moduleRelease, requirements.Kubernetes); err != nil {
			return err
		}
	}

	if len(requirements.ParentModules) > 0 {
		if err := b.ModuleDependency.ValidateRelease(moduleName, moduleRelease, moduleReleaseVersion, requirements.ParentModules); err != nil {
			return err
		}
	}

	return nil
}

func (b *ExtendersStack) IsExtendersField(field string) bool {
	return slices.Contains([]string{
		v1alpha1.KubernetesRequirementFieldName,
		v1alpha1.DeckhouseRequirementFieldName,
		v1alpha1.ModuleDependencyRequirementFieldName,
	}, field)
}
