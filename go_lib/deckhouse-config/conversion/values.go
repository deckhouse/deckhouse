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
	"sigs.k8s.io/yaml"
)

// Settings is a helper to simplify module settings manipulation in conversion functions.
// Access and update map[string]interface{} is difficult, so gjson and sjson come to the rescue.
type Settings struct {
	m         sync.RWMutex
	jsonBytes []byte
}

func SettingsFromMap(in map[string]interface{}) (*Settings, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return SettingsFromBytes(data), nil
}

func SettingsFromBytes(bytes []byte) *Settings {
	return &Settings{
		jsonBytes: bytes,
	}
}

func SettingsFromString(jsonStr string) *Settings {
	return SettingsFromBytes([]byte(jsonStr))
}

func SettingsFromYAML(yamlStr string) (*Settings, error) {
	var settings map[string]interface{}

	err := yaml.Unmarshal([]byte(yamlStr), &settings)
	if err != nil {
		return nil, fmt.Errorf("unmarshal settings from YAML: %v", err)
	}

	return SettingsFromMap(settings)
}

func (s *Settings) Get(path string) gjson.Result {
	s.m.RLock()
	defer s.m.RUnlock()
	return gjson.GetBytes(s.jsonBytes, path)
}

func (s *Settings) Set(path string, value interface{}) error {
	s.m.Lock()
	defer s.m.Unlock()

	newValues, err := sjson.SetBytes(s.jsonBytes, path, value)
	if err != nil {
		return err
	}
	s.jsonBytes = newValues
	return nil
}

func (s *Settings) SetFromJSON(path string, jsonRawValue string) error {
	s.m.Lock()
	defer s.m.Unlock()

	newValues, err := sjson.SetRawBytes(s.jsonBytes, path, []byte(jsonRawValue))
	if err != nil {
		return err
	}
	s.jsonBytes = newValues
	return nil
}

func (s *Settings) Clear() {
	s.m.Lock()
	defer s.m.Unlock()

	s.jsonBytes = []byte("{}")
}

// delete removes field by path without locks.
func (s *Settings) delete(path string) error {
	newValues, err := sjson.DeleteBytes(s.jsonBytes, path)
	if err != nil {
		return err
	}

	s.jsonBytes = newValues
	return nil
}

// isEmpty returns true if value is null, empty array, or empty map.
func (s *Settings) isEmptyNode(path string) bool {
	obj := gjson.GetBytes(s.jsonBytes, path)
	switch {
	case obj.IsArray():
		return len(obj.Array()) == 0
	case obj.IsObject():
		return len(obj.Map()) == 0
	}
	return obj.Value() == nil
}

// IsEmptyNode returns true if value is empty array, or empty map.
func (s *Settings) IsEmptyNode(path string) bool {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.isEmptyNode(path)
}

// Delete removes field by path.
func (s *Settings) Delete(path string) error {
	s.m.Lock()
	defer s.m.Unlock()

	return s.delete(path)
}

// DeleteIfEmptyParent removes field by path if value is an empty object or an empty array.
func (s *Settings) DeleteIfEmptyParent(path string) error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.isEmptyNode(path) {
		return s.delete(path)
	}
	return nil
}

// DeleteAndClean removes path and its empty parents.
// This method supports only dot separated paths. gjson's selectors and modifiers are not supported.
func (s *Settings) DeleteAndClean(path string) error {
	s.m.Lock()
	defer s.m.Unlock()

	err := s.delete(path)
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
		if !s.isEmptyNode(path) {
			break
		}
		err = s.delete(path)
		if err != nil {
			return err
		}
	}
	return nil
}

// Map transforms values into map[string]interface{} object.
func (s *Settings) Map() (map[string]interface{}, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	var m map[string]interface{}

	err := json.Unmarshal(s.jsonBytes, &m)
	if err != nil {
		return nil, fmt.Errorf("json values to map: %s\n%s", err, string(s.jsonBytes))
	}
	return m, nil
}

// Bytes returns underlying json text.
func (s *Settings) Bytes() []byte {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.jsonBytes
}

// Bytes returns underlying json text as string.
func (s *Settings) String() string {
	s.m.RLock()
	defer s.m.RUnlock()
	return string(s.jsonBytes)
}
