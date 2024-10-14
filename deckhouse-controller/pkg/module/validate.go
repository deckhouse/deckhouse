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
	"errors"
	"fmt"
	"path/filepath"

	openapierrors "github.com/go-openapi/errors"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/hashicorp/go-multierror"
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

func Validate(def DeckhouseModuleDefinition, values addonutils.Values) error {
	if def.Weight < 900 || def.Weight > 999 {
		return fmt.Errorf("external module weight must be between 900 and 999")
	}
	if def.Path == "" {
		return fmt.Errorf("cannot validate module without path. Path is required to load openapi specs")
	}

	cb, vb, err := addonutils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
	if err != nil {
		return fmt.Errorf("read open API files: %w", err)
	}
	dm, err := addonmodules.NewBasicModule(def.Name, def.Path, def.Weight, nil, cb, vb)
	if err != nil {
		return fmt.Errorf("new deckhouse module: %w", err)
	}

	if values != nil {
		dm.SaveConfigValues(values)
	}

	err = dm.Validate()
	// Next we will need to record all validation errors except required (602).
	var result, mErr *multierror.Error
	if errors.As(err, &mErr) {
		for _, me := range mErr.Errors {
			var e *openapierrors.Validation
			if errors.As(me, &e) {
				if e.Code() == 602 {
					continue
				}
			}
			result = multierror.Append(result, me)
		}
	}
	// Now result will contain all validation errors, if any, except required.

	if result != nil {
		return fmt.Errorf("validate module: %w", result)
	}

	return nil
}
