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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ModuleSettings is a helper to simplify module settings manipulation in conversion functions.
// Access and update map[string]interface{} is difficult, so gjson and sjson come to the rescue.
type ModuleSettings struct {
	m         sync.RWMutex
	jsonBytes []byte
}

func ModuleSettingsFromMap(in map[string]interface{}) (*ModuleSettings, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return ModuleSettingsFromBytes(data), nil
}

func ModuleSettingsFromBytes(bytes []byte) *ModuleSettings {
	return &ModuleSettings{
		jsonBytes: bytes,
	}
}

func ModuleSettingsFromString(jsonStr string) *ModuleSettings {
	return ModuleSettingsFromBytes([]byte(jsonStr))
}

func (v *ModuleSettings) Get(path string) gjson.Result {
	v.m.RLock()
	defer v.m.RUnlock()
	return gjson.GetBytes(v.jsonBytes, path)
}

func (v *ModuleSettings) Set(path string, value interface{}) error {
	v.m.Lock()
	defer v.m.Unlock()

	newValues, err := sjson.SetBytes(v.jsonBytes, path, value)
	if err != nil {
		return err
	}
	v.jsonBytes = newValues
	return nil
}

func (v *ModuleSettings) SetFromJSON(path string, jsonRawValue string) error {
	v.m.Lock()
	defer v.m.Unlock()

	newValues, err := sjson.SetRawBytes(v.jsonBytes, path, []byte(jsonRawValue))
	if err != nil {
		return err
	}
	v.jsonBytes = newValues
	return nil
}

// delete removes field by path without locks.
func (v *ModuleSettings) delete(path string) error {
	newValues, err := sjson.DeleteBytes(v.jsonBytes, path)
	if err != nil {
		return err
	}

	v.jsonBytes = newValues
	return nil
}

// isEmpty returns true if value is null, empty array, or empty map.
func (v *ModuleSettings) isEmptyParent(path string) bool {
	obj := gjson.GetBytes(v.jsonBytes, path)
	switch {
	case obj.IsArray():
		if len(obj.Array()) == 0 {
			return true
		}
	case obj.IsObject():
		if len(obj.Map()) == 0 {
			return true
		}
	}
	return false
}

// IsEmptyParent returns true if value is empty array, or empty map.
func (v *ModuleSettings) IsEmptyParent(path string) bool {
	v.m.RLock()
	defer v.m.RUnlock()
	return v.isEmptyParent(path)
}

// Delete removes field by path.
func (v *ModuleSettings) Delete(path string) error {
	v.m.Lock()
	defer v.m.Unlock()

	return v.delete(path)
}

// DeleteIfEmptyParent removes field by path if value is an empty object or an empty array.
func (v *ModuleSettings) DeleteIfEmptyParent(path string) error {
	v.m.Lock()
	defer v.m.Unlock()

	if v.isEmptyParent(path) {
		return v.delete(path)
	}
	return nil
}

// DeleteAndClean removes path and its empty parents.
// This method supports only dot separated paths. gjson's selectors and modifiers are not supported.
func (v *ModuleSettings) DeleteAndClean(path string) error {
	v.m.Lock()
	defer v.m.Unlock()

	err := v.delete(path)
	if err != nil {
		return err
	}

	parts := strings.Split(path, ".")
	// Drop parts one by one and check for emptiness.
	for {
		if len(parts) <= 1 {
			break
		}
		parts = parts[0 : len(parts)-1]

		path = strings.Join(parts, ".")
		if !v.isEmptyParent(path) {
			break
		}
		err = v.delete(path)
		if err != nil {
			return err
		}
	}
	return nil
}

// Map transforms values into map[string]interface{} object.
func (v *ModuleSettings) Map() (map[string]interface{}, error) {
	v.m.RLock()
	defer v.m.RUnlock()

	var m map[string]interface{}

	err := json.Unmarshal(v.jsonBytes, &m)
	if err != nil {
		return nil, fmt.Errorf("json values to map: %s\n%s", err, string(v.jsonBytes))
	}
	return m, nil
}

// Bytes returns underlying json text.
func (v *ModuleSettings) Bytes() []byte {
	v.m.RLock()
	defer v.m.RUnlock()
	return v.jsonBytes
}

// Bytes returns underlying json text as string.
func (v *ModuleSettings) String() string {
	v.m.RLock()
	defer v.m.RUnlock()
	return string(v.jsonBytes)
}
