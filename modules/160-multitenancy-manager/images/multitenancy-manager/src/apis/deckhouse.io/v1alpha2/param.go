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

package v1alpha2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Param is a leaf field of a schema-based ProjectTemplate that is either a literal value of type T
// or a reference to a Project parameter: {fromParam: "name"}.
//
// This is the "union-type on leaves" parametrization. A template declares its structure with
// typed literal values, and any individual leaf may instead defer its value to a per-project
// parameter (declared and validated by spec.parametersSchema). The built-in templates use fromParam
// for every overridable field so they keep exactly the per-project surface they had as v1alpha1
// resourcesTemplate; bespoke templates can mix literals and fromParam freely.
//
// The custom UnmarshalJSON is deliberately tolerant: a typed client Get on a ProjectTemplate must
// succeed whether a field holds a literal or a {fromParam} object, so neither form may error during
// decoding. The CRD marks fromParam-capable fields with x-kubernetes-preserve-unknown-fields so the
// API server stores both forms.
type Param[T any] struct {
	value     *T
	fromParam string
}

// LiteralParam builds a Param holding a literal value (mainly for tests and built-in defaults).
func LiteralParam[T any](v T) Param[T] { return Param[T]{value: &v} }

// FromParamRef builds a Param referencing a Project parameter by (optionally dotted) name.
func FromParamRef[T any](name string) Param[T] { return Param[T]{fromParam: name} }

// IsZero reports whether the Param carries neither a literal nor a reference.
func (p Param[T]) IsZero() bool { return p.value == nil && p.fromParam == "" }

// Ref returns the referenced parameter name, or "" when the Param is a literal or empty.
func (p Param[T]) Ref() string { return p.fromParam }

func (p *Param[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*p = Param[T]{}
		return nil
	}

	// A single-key object {"fromParam": "name"} is always a parameter reference. This is
	// unambiguous for the literal types we use (string, bool, int64, []Toleration, IDRange); the only
	// theoretical clash is a map literal whose sole key is "fromParam", which is documented as reserved.
	if data[0] == '{' {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(data, &probe); err == nil {
			if raw, ok := probe["fromParam"]; ok && len(probe) == 1 {
				var name string
				if err := json.Unmarshal(raw, &name); err != nil {
					return fmt.Errorf("fromParam must be a string: %w", err)
				}
				*p = Param[T]{fromParam: name}
				return nil
			}
		}
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = Param[T]{value: &v}
	return nil
}

func (p Param[T]) MarshalJSON() ([]byte, error) {
	if p.fromParam != "" {
		return json.Marshal(map[string]string{"fromParam": p.fromParam})
	}
	if p.value != nil {
		return json.Marshal(*p.value)
	}
	return []byte("null"), nil
}

// DeepCopyParam returns a deep copy via a JSON round-trip, which is adequate for the JSON-serializable
// leaf types Param is instantiated with.
func (p Param[T]) DeepCopyParam() Param[T] {
	out := Param[T]{fromParam: p.fromParam}
	if p.value != nil {
		if b, err := json.Marshal(*p.value); err == nil {
			var v T
			if json.Unmarshal(b, &v) == nil {
				out.value = &v
			}
		}
	}
	return out
}

// Resolve returns the effective value of the Param against the merged Project parameters. The bool
// is false when the field is unset: no literal, and (for a reference) the parameter is absent or
// null. A referenced parameter that cannot be decoded into T is reported as an error.
func (p Param[T]) Resolve(params map[string]any) (T, bool, error) {
	var zero T
	if p.fromParam != "" {
		raw, found := LookupParam(params, p.fromParam)
		if !found || raw == nil {
			return zero, false, nil
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return zero, false, fmt.Errorf("marshal parameter %q: %w", p.fromParam, err)
		}
		var v T
		if err := json.Unmarshal(b, &v); err != nil {
			return zero, false, fmt.Errorf("parameter %q is not assignable to this field: %w", p.fromParam, err)
		}
		return v, true, nil
	}
	if p.value != nil {
		return *p.value, true, nil
	}
	return zero, false, nil
}

// LookupParam resolves a (optionally dotted) parameter path against a parameters map, e.g.
// "namespace.labels" -> params["namespace"]["labels"].
func LookupParam(params map[string]any, path string) (any, bool) {
	if params == nil || path == "" {
		return nil, false
	}
	cur := any(params)
	for _, segment := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
