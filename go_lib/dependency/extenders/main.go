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

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

func IsExtendersField(field string) bool {
	return slices.Contains([]string{
		kubernetesversion.RequirementsField,
		deckhouseversion.RequirementsField,
		bootstrapped.RequirementsField,
		moduledependency.RequirementsField,
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

func AddConstraints(module string, requirements map[string]interface{}) error {
	if len(requirements[deckhouseversion.RequirementsField]) > 0 {
		if err := deckhouseversion.Instance().AddConstraint(module, requirements[deckhouseversion.RequirementsField]); err != nil {
			return err
		}
	}
	if len(requirements[kubernetesversion.RequirementsField]) > 0 {
		if err := kubernetesversion.Instance().AddConstraint(module, requirements[kubernetesversion.RequirementsField]); err != nil {
			return err
		}
	}
	if len(requirements[bootstrapped.RequirementsField]) > 0 {
		if err := bootstrapped.Instance().AddConstraint(module, requirements[bootstrapped.RequirementsField]); err != nil {
			return err
		}
	}
	if len(requirements[moduledependency.RequirementsField]) > 0 {
		if err := moduledependency.Instance().AddConstraint(module, requirements[moduledependency.RequirementsField]); err != nil {
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

func CheckModuleReleaseRequirements(moduleRelease string, requirements map[string]interface{}) error {
	if len(requirements[kubernetesversion.RequirementsField]) > 0 {
		if err := kubernetesversion.Instance().ValidateRelease(moduleRelease, requirements[kubernetesversion.RequirementsField]); err != nil {
			return err
		}
	}
	if len(requirements[deckhouseversion.RequirementsField]) > 0 {
		if err := deckhouseversion.Instance().ValidateRelease(moduleRelease, requirements[deckhouseversion.RequirementsField]); err != nil {
			return err
		}
	}
	if len(requirements[moduledependency.RequirementsField]) > 0 {
		if err := deckhouseversion.Instance().ValidateRelease(moduleRelease, requirements[moduledependency.RequirementsField]); err != nil {
			return err
		}
	}

	return nil
}
