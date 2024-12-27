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
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

func IsExtendersField(field string) bool {
	return slices.Contains([]string{
		v1alpha1.KubernetesRequirementFieldName,
		v1alpha1.DeckhouseRequirementFieldName,
		v1alpha1.BootstrappedRequirementFieldName,
		v1alpha1.ModuleDependencyRequirementFieldName,
	}, field)
}

func Extenders() []extenders.Extender {
	return []extenders.Extender{
		kubernetesversion.Instance(),
		deckhouseversion.Instance(),
		bootstrapped.Instance(),
		moduledependency.Instance(),
	}
}

func AddConstraints(module string, requirements *v1alpha1.ModuleRequirements) error {
	if requirements == nil {
		// no requirements
		return nil
	}

	if len(requirements.Deckhouse) > 0 {
		if err := deckhouseversion.Instance().AddConstraint(module, requirements.Deckhouse); err != nil {
			return err
		}
	}

	if len(requirements.Kubernetes) > 0 {
		if err := kubernetesversion.Instance().AddConstraint(module, requirements.Kubernetes); err != nil {
			return err
		}
	}

	if len(requirements.Bootstrapped) > 0 {
		if err := bootstrapped.Instance().AddConstraint(module, requirements.Bootstrapped); err != nil {
			return err
		}
	}

	if len(requirements.ParentModules) > 0 {
		if err := moduledependency.Instance().AddConstraint(module, requirements.ParentModules); err != nil {
			return err
		}
	}

	return nil
}

func DeleteConstraints(module string) {
	deckhouseversion.Instance().DeleteConstraint(module)
	kubernetesversion.Instance().DeleteConstraint(module)
	bootstrapped.Instance().DeleteConstraint(module)
	moduledependency.Instance().DeleteConstraint(module)
}

func CheckModuleReleaseRequirements(moduleName, moduleRelease string, moduleReleaseVersion *semver.Version, requirements *v1alpha1.ModuleReleaseRequirements) error {
	if requirements == nil {
		// no requirements
		return nil
	}

	if len(requirements.Deckhouse) > 0 {
		if err := deckhouseversion.Instance().ValidateRelease(moduleRelease, requirements.Deckhouse); err != nil {
			return err
		}
	}

	if len(requirements.Kubernetes) > 0 {
		if err := kubernetesversion.Instance().ValidateRelease(moduleRelease, requirements.Kubernetes); err != nil {
			return err
		}
	}

	if len(requirements.ParentModules) > 0 {
		if err := moduledependency.Instance().ValidateRelease(moduleName, moduleRelease, moduleReleaseVersion, requirements.ParentModules); err != nil {
			return err
		}
	}

	return nil
}
