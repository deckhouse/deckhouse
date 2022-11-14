/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config_values_conversion

import (
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
)

const moduleName = "istio"

var _ = conversion.RegisterFunc(moduleName, 1, 2, convertV1ToV2)

// convertV1ToV2 removes deprecated fields.
func convertV1ToV2(values *conversion.ModuleSettings) error {
	return values.DeleteAndClean("auth.password")
}
