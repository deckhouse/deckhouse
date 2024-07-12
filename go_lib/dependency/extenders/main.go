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
