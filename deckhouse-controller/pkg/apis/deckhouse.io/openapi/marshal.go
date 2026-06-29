// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openapi

import (
	"encoding/json"
	"errors"
)

var jsTrue = []byte("true")
var jsFalse = []byte("false")

func (s OpenAPIV3SchemaOrBool) MarshalJSON() ([]byte, error) {
	if s.Schema != nil {
		return json.Marshal(s.Schema)
	}
	if s.Schema == nil && !s.Allows {
		return jsFalse, nil
	}
	return jsTrue, nil
}

func (s *OpenAPIV3SchemaOrBool) UnmarshalJSON(data []byte) error {
	var nw OpenAPIV3SchemaOrBool
	switch {
	case len(data) == 0:
	case data[0] == '{':
		var sch OpenAPIV3Schema
		if err := json.Unmarshal(data, &sch); err != nil {
			return err
		}
		nw.Allows = true
		nw.Schema = &sch
	case len(data) == 4 && string(data) == "true":
		nw.Allows = true
	case len(data) == 5 && string(data) == "false":
		nw.Allows = false
	default:
		return errors.New("boolean or JSON schema expected")
	}
	*s = nw
	return nil
}

func (s OpenAPIV3SchemaOrStringArray) MarshalJSON() ([]byte, error) {
	if len(s.Property) > 0 {
		return json.Marshal(s.Property)
	}
	if s.Schema != nil {
		return json.Marshal(s.Schema)
	}
	return []byte("null"), nil
}

func (s *OpenAPIV3SchemaOrStringArray) UnmarshalJSON(data []byte) error {
	var first byte
	if len(data) > 1 {
		first = data[0]
	}
	var nw OpenAPIV3SchemaOrStringArray
	if first == '{' {
		var sch OpenAPIV3Schema
		if err := json.Unmarshal(data, &sch); err != nil {
			return err
		}
		nw.Schema = &sch
	}
	if first == '[' {
		if err := json.Unmarshal(data, &nw.Property); err != nil {
			return err
		}
	}
	*s = nw
	return nil
}

func (s OpenAPIV3SchemaOrArray) MarshalJSON() ([]byte, error) {
	if len(s.JSONSchemas) > 0 {
		return json.Marshal(s.JSONSchemas)
	}
	return json.Marshal(s.Schema)
}

func (s *OpenAPIV3SchemaOrArray) UnmarshalJSON(data []byte) error {
	var nw OpenAPIV3SchemaOrArray
	var first byte
	if len(data) > 1 {
		first = data[0]
	}
	if first == '{' {
		var sch OpenAPIV3Schema
		if err := json.Unmarshal(data, &sch); err != nil {
			return err
		}
		nw.Schema = &sch
	}
	if first == '[' {
		if err := json.Unmarshal(data, &nw.JSONSchemas); err != nil {
			return err
		}
	}
	*s = nw
	return nil
}
