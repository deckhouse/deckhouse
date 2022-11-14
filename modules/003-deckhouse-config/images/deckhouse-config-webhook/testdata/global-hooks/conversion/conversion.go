/*
Copyright 2022 Flant JSC

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

package conversion

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
)

var _ = conversion.RegisterFunc("global", 1, 2, convertV1ToV2)

// convertV1ToV2 transforms numeric field to string field.
func convertV1ToV2(values *conversion.ModuleSettings) error {
	newValue := fmt.Sprintf("%d", values.Get("paramNum").Int())
	_ = values.Delete("paramNum")
	_ = values.Set("globalParam", newValue)
	return nil
}
