/*
Copyright 2026 Flant JSC

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

package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Object returns an empty unstructured typed as gvk.
func Object(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

// List returns an empty unstructured list typed as gvk's List kind.
func List(gvk schema.GroupVersionKind) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	return list
}

// AsUnstructured returns obj if it is an unstructured object.
func AsUnstructured(obj client.Object) (*unstructured.Unstructured, bool) {
	u, ok := obj.(*unstructured.Unstructured)
	return u, ok
}

// DecodeInto JSON-roundtrips obj into dest.
func DecodeInto(obj *unstructured.Unstructured, dest any) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	raw, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

// AsInt64 converts JSON-decoded numeric values to int64.
func AsInt64(v any) (int64, error) {
	if v == nil {
		return 0, nil
	}
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		if uint64(n) > uint64(math.MaxInt64) {
			return 0, fmt.Errorf("value %d overflows int64", n)
		}
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		if n > uint64(math.MaxInt64) {
			return 0, fmt.Errorf("value %d overflows int64", n)
		}
		return int64(n), nil
	case float64:
		if n > float64(math.MaxInt64) || n < float64(math.MinInt64) {
			return 0, fmt.Errorf("value %v overflows int64", n)
		}
		if math.Trunc(n) != n {
			return 0, fmt.Errorf("value %v is not an integer", n)
		}
		return int64(n), nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("parse: %w", err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("unexpected type %T", v)
	}
}

// ParseRFC3339 parses value as RFC3339 or RFC3339Nano. Empty input is a nil time.
func ParseRFC3339(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", value, err)
		}
	}
	utc := parsed.UTC()
	return &utc, nil
}
