/*
Copyright 2023 Flant JSC

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

package module

import (
	"fmt"

	addonutils "github.com/flant/addon-operator/pkg/utils"
)

func ValidateDefinition(def DeckhouseModuleDefinition) error {
	if def.Weight < 900 || def.Weight > 999 {
		return fmt.Errorf("external module weight must be between 900 and 999")
	}

	if def.Path == "" {
		return fmt.Errorf("cannot validate module without path. Path is required to load openapi specs")
	}

	dm, err := NewDeckhouseModule(def, addonutils.Values{}, nil, nil)
	if err != nil {
		return fmt.Errorf("new deckhouse module: %w", err)
	}

	err = dm.GetBasicModule().Validate()
	if err != nil {
		return fmt.Errorf("validate module: %w", err)
	}

	return nil
}
