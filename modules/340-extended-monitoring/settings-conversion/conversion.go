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
		err := settings.Set("imageAvailability.registry.tlsConfig.insecureSkipVerify", insecure)
		if err != nil {
			return err
		}
	}

	return settings.Delete("imageAvailability.skipRegistryCertVerification")
}
