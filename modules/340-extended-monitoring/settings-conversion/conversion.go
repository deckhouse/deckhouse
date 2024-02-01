/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package settings_conversion

import (
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
)

const moduleName = "extended-monitoring"

var _ = conversion.RegisterFunc(moduleName, 1, 2, convertV1ToV2)

// convertV1ToV2 removes the deprecated field and preserves its value in a new field
func convertV1ToV2(settings *conversion.Settings) error {
	insecure := settings.Get("imageAvailability.skipRegistryCertVerification").Bool()
	if insecure {
		err := settings.Set("imageAvailability.tlsConfig.insecureSkipVerify", insecure)
		if err != nil {
			return err
		}
	}

	return settings.DeleteAndClean("imageAvailability.skipRegistryCertVerification")
}
