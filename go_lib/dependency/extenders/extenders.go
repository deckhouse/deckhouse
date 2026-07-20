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
	"fmt"
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

type IExtendersStack interface {
	GetExtenders() []extenders.Extender
	AddConstraints(module string, critical bool, access *moduletypes.ModuleAccessibility, requirements *v1alpha1.ModuleRequirements) error
	DeleteConstraints(module string)
	CheckModuleReleaseRequirements(moduleName, moduleRelease string, moduleReleaseVersion *semver.Version, requirements *v1alpha1.ModuleReleaseRequirements) error
	IsExtendersField(field string) bool

	GetDeckhouseVersion() deckhouseversion.IExtender
	GetKubernetesVersion() kubernetesversion.IExtender
	GetModuleDependency() moduledependency.IExtender
	GetBootstrapped() bootstrapped.IExtender
	GetEditionAvailable() editionavailable.IExtender
	GetEditionEnabled() editionenabled.IExtender
}

var _ IExtendersStack = &ExtendersStack{}

type ExtendersStack struct {
	deckhouseVersion  deckhouseversion.IExtender
	kubernetesVersion kubernetesversion.IExtender
	moduleDependency  moduledependency.IExtender
	bootstrapped      bootstrapped.IExtender
	editionAvailable  editionavailable.IExtender
	editionEnabled    editionenabled.IExtender
}

func NewExtendersStack(edition *d8edition.Edition, bootstrappedHelper func() (bool, error), logger *log.Logger) *ExtendersStack {
	return &ExtendersStack{
		deckhouseVersion:  deckhouseversion.NewExtender(edition.Version, logger.Named("deckhouse-version-extender")),
		kubernetesVersion: kubernetesversion.Instance(),
		moduleDependency:  moduledependency.Instance(),
		bootstrapped:      bootstrapped.NewExtender(bootstrappedHelper, logger.Named("bootstrapped-extender")),
		editionAvailable:  editionavailable.New(edition.Name, logger.Named("edition-available-extender")),
		editionEnabled:    editionenabled.New(edition.Name, edition.Bundle, logger.Named("edition-enabled-extender")),
	}
}

func (b *ExtendersStack) GetDeckhouseVersion() deckhouseversion.IExtender {
	return b.deckhouseVersion
}

func (b *ExtendersStack) GetKubernetesVersion() kubernetesversion.IExtender {
	return b.kubernetesVersion
}

func (b *ExtendersStack) GetModuleDependency() moduledependency.IExtender {
	return b.moduleDependency
}

func (b *ExtendersStack) GetBootstrapped() bootstrapped.IExtender {
	return b.bootstrapped
}

func (b *ExtendersStack) GetEditionAvailable() editionavailable.IExtender {
	return b.editionAvailable
}

func (b *ExtendersStack) GetEditionEnabled() editionenabled.IExtender {
	return b.editionEnabled
}

func (b *ExtendersStack) GetExtenders() []extenders.Extender {
	return []extenders.Extender{
		b.deckhouseVersion,
		b.kubernetesVersion,
		b.moduleDependency,
		b.bootstrapped,
		b.editionAvailable,
		b.editionEnabled,
	}
}

func (b *ExtendersStack) AddConstraints(module string, critical bool, access *moduletypes.ModuleAccessibility, requirements *v1alpha1.ModuleRequirements) error {
	if !critical {
		b.bootstrapped.AddFunctionalModule(module)
	}

	if access != nil {
		b.editionEnabled.AddModule(module, access)
		b.editionAvailable.AddModule(module, access)
	}

	if requirements == nil {
		// no requirements
		return nil
	}

	if len(requirements.Deckhouse) > 0 {
		if err := b.deckhouseVersion.AddConstraint(module, requirements.Deckhouse); err != nil {
			return fmt.Errorf("add constraint: %w", err)
		}
	}

	if len(requirements.Kubernetes) > 0 {
		if err := b.kubernetesVersion.AddConstraint(module, requirements.Kubernetes); err != nil {
			return fmt.Errorf("add constraint: %w", err)
		}
	}

	if len(requirements.ParentModules) > 0 {
		if err := b.moduleDependency.AddConstraint(module, requirements.ParentModules); err != nil {
			return fmt.Errorf("add constraint: %w", err)
		}
	}

	return nil
}

func (b *ExtendersStack) DeleteConstraints(module string) {
	b.deckhouseVersion.DeleteConstraint(module)
	b.kubernetesVersion.DeleteConstraint(module)
	b.moduleDependency.DeleteConstraint(module)
}

func (b *ExtendersStack) CheckModuleReleaseRequirements(moduleName, moduleRelease string, moduleReleaseVersion *semver.Version, requirements *v1alpha1.ModuleReleaseRequirements) error {
	if requirements == nil {
		// no requirements
		return nil
	}

	if len(requirements.Deckhouse) > 0 {
		if err := b.deckhouseVersion.ValidateRelease(moduleRelease, requirements.Deckhouse); err != nil {
			return fmt.Errorf("validate release: %w", err)
		}
	}

	if len(requirements.Kubernetes) > 0 {
		if err := b.kubernetesVersion.ValidateRelease(moduleRelease, requirements.Kubernetes); err != nil {
			return fmt.Errorf("validate release: %w", err)
		}
	}

	if len(requirements.ParentModules) > 0 {
		if err := b.moduleDependency.ValidateRelease(moduleName, moduleRelease, moduleReleaseVersion, requirements.ParentModules); err != nil {
			return fmt.Errorf("validate release: %w", err)
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
