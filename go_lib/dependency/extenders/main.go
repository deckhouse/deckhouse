package extenders

import (
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
)

func IsExtendersField(field string) bool {
	return slices.Contains([]string{kubernetesversion.RequirementsField, deckhouseversion.RequirementsField}, field)
}

func Extenders() []extenders.Extender {
	return []extenders.Extender{
		kubernetesversion.Instance(),
		deckhouseversion.Instance(),
	}
}

func AddConstraints(module string, requirements map[string]string) error {
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
	return nil
}

func CheckRequirements(moduleRelease string, requirements map[string]string) error {
	if len(requirements[kubernetesversion.RequirementsField]) > 0 {
		if err := kubernetesversion.Instance().ValidateConstraint(moduleRelease, requirements[kubernetesversion.RequirementsField]); err != nil {
			return err
		}
	}
	if len(requirements[deckhouseversion.RequirementsField]) > 0 {
		if err := deckhouseversion.Instance().ValidateConstraint(moduleRelease, requirements[deckhouseversion.RequirementsField]); err != nil {
			return err
		}
	}
	return nil
}
