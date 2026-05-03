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

package cel

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/types/known/structpb"
)

// ruleKey is the OpenAPI extension key that carries CEL validation rules for Deckhouse schemas.
const ruleKey = "x-deckhouse-validations"

// rule is a single CEL validation entry in an x-deckhouse-validations extension list.
// Expression is a CEL expression over "self"; Message is returned when the expression is false.
type rule struct {
	Expression string `json:"expression" yaml:"expression"`
	Message    string `json:"message" yaml:"message"`
}

// Validate evaluates all x-deckhouse-validations CEL rules attached to schema and its
// nested properties recursively. It returns validation errors for every rule that
// evaluated to false, and a non-nil error for configuration or evaluation failures.
func Validate(schema *spec.Schema, values any) ([]error, error) {
	var validationErrs []error
	// First we validate only object nested properties
	if m, ok := values.(map[string]any); ok {
		for propName, propSchema := range schema.Properties {
			if propValue, ok := m[propName]; ok {
				subErrs, err := Validate(&propSchema, propValue)
				if err != nil {
					return nil, err
				}
				validationErrs = append(validationErrs, subErrs...)
			}
		}
	}

	raw, found := schema.Extensions[ruleKey]
	if !found {
		if len(validationErrs) > 0 {
			return validationErrs, nil
		}
		return nil, nil
	}

	var rules []rule
	switch v := raw.(type) {
	case []any:
		for _, entry := range v {
			mapEntry, ok := entry.(map[string]any)
			if !ok || len(mapEntry) == 0 {
				return nil, fmt.Errorf("x-deckhouse-validations invalid")
			}

			if val, ok := mapEntry["expression"]; !ok || len(val.(string)) == 0 {
				return nil, fmt.Errorf("x-deckhouse-validations invalid: missing expression")
			}
			if val, ok := mapEntry["message"]; !ok || len(val.(string)) == 0 {
				return nil, fmt.Errorf("x-deckhouse-validations invalid: missing message")
			}

			rules = append(rules, rule{
				Expression: mapEntry["expression"].(string),
				Message:    mapEntry["message"].(string),
			})
		}
	default:
		return nil, fmt.Errorf("x-deckhouse-validations invalid")
	}

	celSelfType, celSelfValue, err := buildCELValueAndType(values)
	if err != nil {
		return nil, err
	}

	env, err := cel.NewEnv(cel.Variable("self", celSelfType))
	if err != nil {
		return nil, fmt.Errorf("create CEL env: %w", err)
	}
	for _, r := range rules {
		ast, issues := env.Compile(r.Expression)
		if issues.Err() != nil {
			return nil, fmt.Errorf("compile the '%s' rule: %w", r.Expression, issues.Err())
		}

		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("create program for the '%s' rule: %w", r.Expression, err)
		}

		out, _, err := prg.Eval(map[string]any{"self": celSelfValue})
		if err != nil {
			if strings.Contains(err.Error(), "no such key:") {
				continue
			}
			return nil, fmt.Errorf("evaluate the '%s' rule: %w", r.Expression, err)
		}

		pass, ok := out.Value().(bool)
		if !ok {
			return nil, errors.New("rule should return boolean")
		}
		if !pass {
			validationErrs = append(validationErrs, errors.New(r.Message))
		}
	}

	return validationErrs, nil
}

// buildCELValueAndType converts a Go value to the CEL type descriptor and
// protobuf-compatible value required by cel.Program.Eval.
func buildCELValueAndType(value any) (*cel.Type, any, error) {
	switch v := value.(type) {
	case map[string]any:
		obj, err := structpb.NewStruct(v)
		if err != nil {
			return nil, nil, fmt.Errorf("convert values to struct: %w", err)
		}
		return cel.MapType(cel.StringType, cel.DynType), obj, nil
	case []any:
		list, err := structpb.NewList(v)
		if err != nil {
			return nil, nil, fmt.Errorf("convert array to list: %w", err)
		}
		return cel.ListType(cel.DynType), list, nil
	default:
		val, err := structpb.NewValue(v)
		if err != nil {
			return nil, nil, fmt.Errorf("convert dyn to value: %w", err)
		}
		return cel.DynType, val, nil
	}
}
