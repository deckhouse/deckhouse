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

	data, err := DecodeDataFromSecret(obj)
	if err != nil {
		t.Fatalf("data should be decoded: %v", err)
	}

	var v interface{}

	v = data["simple"]
	if _, ok := v.(string); !ok {
		t.Fatalf(`data["simple"] should be string. Got type=%T val=%#v`, v, v)
	}

	v = data["json_array"]
	if _, ok := v.([]interface{}); !ok {
		t.Fatalf(`data["json_array"] should be []interface{}. Got type=%T val=%#v`, v, v)
	}

	v = data["json_object"]
	if _, ok := v.(map[string]interface{}); !ok {
		t.Fatalf(`data["json_object"] should be map[string]interface{}. Got type=%T val=%#v`, v, v)
	}

	v = data["json_string"]
	if _, ok := v.(string); !ok {
		t.Fatalf(`data["json_string"] should be string. Got type=%T val=%#v`, v, v)
	}

	v = data["json_number"]
	if _, ok := v.(string); !ok {
		t.Fatalf(`data["json_number"] should not be converted from string. Got type=%T val=%#v`, v, v)
	}
}
