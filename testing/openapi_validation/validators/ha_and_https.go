/*
Copyright 2021 Flant JSC

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

package validators

import (
	"fmt"
	"reflect"
)

var (
	absoluteKeysExcludes = map[string]string{
		"modules/150-user-authn/openapi/config-values.yaml": "properties.publishAPI.properties.https",
		"global-hooks/openapi/config-values.yaml":           "properties.modules.properties.https",
	}
)

type HAValidator struct {
}

func NewHAValidator() HAValidator {
	return HAValidator{}
}

func (en HAValidator) Run(file, absoluteKey string, value interface{}) error {
	values, ok := value.(map[interface{}]interface{})
	if !ok {
		fmt.Printf("Possible Bug? Have to be a map. Type: %s, Value: %s, File: %s, Key: %s\n", reflect.TypeOf(value), value, file, absoluteKey)
		return nil
	}

	for key := range values {
		if key == "default" {
			if absoluteKeysExcludes[file] == absoluteKey {
				continue
			}
			return fmt.Errorf("%s is invalid: must have no default value", absoluteKey)
		}
	}

	return nil
}
