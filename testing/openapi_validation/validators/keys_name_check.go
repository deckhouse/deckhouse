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

package validators

import (
	"fmt"
	"reflect"
)

var (
	bannedNames = []string{"x-examples", "examples", "example"}
)

type KeyNameValidator struct {
}

func NewKeyNameValidator() KeyNameValidator {
	return KeyNameValidator{}
}

func checkMapForBannedKey(m map[interface{}]interface{}, banned []string) error {
	for k, v := range m {
		if strKey, ok := k.(string); ok {
			for _, ban := range banned {
				if strKey == ban {
					return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
				}
			}
		}
		if nestedMap, ok := v.(map[interface{}]interface{}); ok {
			err := checkMapForBannedKey(nestedMap, banned)
			if err != nil {
				return err
			}
		}
		if nestedSlise, ok := v.([]interface{}); ok {
			for _, item := range nestedSlise {
				if strKey, ok := item.(string); ok {
					for _, ban := range banned {
						if strKey == ban {
							return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
						}
					}
				}
				if nestedMap, ok := item.(map[interface{}]interface{}); ok {
					err := checkMapForBannedKey(nestedMap, banned)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (knv KeyNameValidator) Run(file, _ string, value interface{}) error {
	object, ok := value.(map[interface{}]interface{})
	if !ok {
		fmt.Println("Possible Bug? Have to be a map", reflect.TypeOf(value))
		return nil
	}
	err := checkMapForBannedKey(object, bannedNames)
	if err != nil {
		return fmt.Errorf("%s file validation error: wrong property: %w", file, err)
	}
	return nil
}
