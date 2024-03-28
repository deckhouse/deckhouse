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

package validators

import (
	"fmt"
	"reflect"
)

type ExtendedCRDValidator struct {
	parentCRD map[interface{}]interface{}
}

func NewExtendedCRDValidator() *ExtendedCRDValidator {
	return &ExtendedCRDValidator{}
}

func (excv *ExtendedCRDValidator) WithParentCRD(parentCRD map[interface{}]interface{}) *ExtendedCRDValidator {
	excv.parentCRD = parentCRD
	return excv
}

func (excv *ExtendedCRDValidator) checkIfObjectIsSubset(childCRD map[interface{}]interface{}) error {
	if !isSubset(childCRD, excv.parentCRD) {
		return fmt.Errorf("CRD is not subset")
	}
	return nil
}

func (excv ExtendedCRDValidator) Run(file, _ string, value interface{}) error {
	object, ok := value.(map[interface{}]interface{})
	if !ok {
		fmt.Println("Possible Bug? Have to be a map", reflect.TypeOf(value))
		return nil
	}
	err := excv.checkIfObjectIsSubset(object)
	if err != nil {
		return fmt.Errorf("%s file validation error: wrong property: %w", file, err)
	}
	return nil
}

func isSubset(mapChild, mapParent map[interface{}]interface{}) bool {
	for key, valueChild := range mapChild {
		if valueParent, ok := mapParent[key]; ok {
			switch valueChildConverted := valueChild.(type) {
			case map[interface{}]interface{}:
				if valueParentConverted, ok := valueParent.(map[interface{}]interface{}); ok {
					if !isSubset(valueChildConverted, valueParentConverted) {
						return false
					}
				} else {
					// value of mapParent is not map[interface{}]interface{}
					return false
				}
			case []map[interface{}]interface{}:
				if valueParentConverted, ok := valueParent.([]map[interface{}]interface{}); ok {
					if len(valueChildConverted) != len(valueParentConverted) {
						return false
					}
					for i := range valueChildConverted {
						if !isSubset(valueChildConverted[i], valueParentConverted[i]) {
							return false
						}
					}
				} else {
					return false
				}
			case []interface{}:
				if valueParentConverted, ok := valueParent.([]interface{}); ok {
					if len(valueChildConverted) > len(valueParentConverted) {
						return false
					}
					for i := range valueChildConverted {
						switch valueChildNested := valueChildConverted[i].(type) {
						case map[interface{}]interface{}:
							if valueParentNested, ok := valueParentConverted[i].(map[interface{}]interface{}); ok {
								if !isSubset(valueChildNested, valueParentNested) {
									return false
								}
							} else {
								return false
							}
						default:
							if reflect.TypeOf(valueChildConverted[i]) != reflect.TypeOf(valueParentConverted[i]) {
								return false
							}
						}
					}
				} else {
					return false
				}
			default:
				// TypeOf for not map[interface{}]interface{}
				if reflect.TypeOf(valueChild) != reflect.TypeOf(valueParent) {
					return false
				}
			}
		} else {
			// key from mapChild not founded in mapParent
			return false
		}
	}
	// if we are here all passed
	return true
}
