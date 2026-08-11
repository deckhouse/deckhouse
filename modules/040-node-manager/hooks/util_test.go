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

package hooks

import (
	"testing"

	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
)

func Test_DecodeDataFromSecret(t *testing.T) {
	secret := new(v1.Secret)
	secret.Data = map[string][]byte{
		"simple":      []byte("simple string"),
		"json_array":  []byte(`["a", "b", "c"]`),
		"json_object": []byte(`{"a":1, "b":2, "c":3}`),
		"json_string": []byte(`"json_string"`),
		"json_number": []byte(`0.12`),
	}
	obj, err := sdk.ToUnstructured(secret)
	if err != nil {
		t.Fatalf("secret should be converted to unstructured: %v", err)
	}

	data, err := decodeDataFromSecret(obj)
	if err != nil {
		t.Fatalf("data should be decoded: %v", err)
	}

	var v interface{}

	// Secret data has string value.
	v = data["simple"]
	if _, ok := v.(string); !ok {
		t.Fatalf(`data["simple"] should be string. Got type=%T val=%#v`, v, v)
	}

	// Secret data has JSON-encoded array value.
	v = data["json_array"]
	if _, ok := v.([]interface{}); !ok {
		t.Fatalf(`data["json_array"] should be []interface{}. Got type=%T val=%#v`, v, v)
	}

	// Secret data has JSON-encoded object value.
	v = data["json_object"]
	if _, ok := v.(map[string]interface{}); !ok {
		t.Fatalf(`data["json_object"] should be map[string]interface{}. Got type=%T val=%#v`, v, v)
	}

	// Secret data has JSON-encoded string value.
	v = data["json_string"]
	if _, ok := v.(string); !ok {
		t.Fatalf(`data["json_string"] should be string. Got type=%T val=%#v`, v, v)
	}

	// Secret data has JSON-encoded number value.
	v = data["json_number"]
	if _, ok := v.(string); !ok {
		t.Fatalf(`data["json_number"] should not be converted from string. Got type=%T val=%#v`, v, v)
	}
}
