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
	celast "github.com/google/cel-go/common/ast"
	"google.golang.org/protobuf/types/known/structpb"
)

// ruleKey is the OpenAPI extension key that carries CEL validation rules for Deckhouse schemas.
const ruleKey = "x-deckhouse-validations"

// selfVar / oldSelfVar are the CEL variable names exposed to expressions.
// selfVar holds the current value at the schema level being evaluated;
// oldSelfVar holds the corresponding previous value (e.g. the previously
// stored ModuleConfig settings) and powers transition rules / immutability checks.
const (
	selfVar    = "self"
	oldSelfVar = "oldSelf"
)

// rule is a single CEL validation entry in an x-deckhouse-validations extension list.
// Expression is a CEL expression over "self" (and optionally "oldSelf");
// Message is returned when the expression is false.
type rule struct {
	Expression string `json:"expression" yaml:"expression"`
	Message    string `json:"message" yaml:"message"`
}

// Validate evaluates all x-deckhouse-validations CEL rules attached to schema and
// its nested properties recursively without any previous-value context. Rules
// that reference oldSelf (transition rules) are skipped.
//
// It is equivalent to ValidateTransition(schema, values, nil) and is preserved
// as a thin wrapper for callers that don't have access to the previous values.
func Validate(schema *spec.Schema, values any) ([]error, error) {
	return ValidateTransition(schema, values, nil)
}

// ValidateTransition evaluates all x-deckhouse-validations CEL rules attached to
// schema and its nested properties recursively, exposing the current value as
// "self" and the previous value as "oldSelf" inside expressions.
//
// Rules referencing oldSelf are treated as transition rules: they are
// evaluated only when an old value is available at the same level (i.e. on
// updates) and silently skipped otherwise (e.g. on create or for newly added
// properties). This mirrors the semantics of x-kubernetes-validations and
// allows expressing immutability with a rule like:
//
//	x-deckhouse-validations:
//	  - expression: "self == oldSelf"
//	    message: "field is immutable"
//
// It returns validation errors for every rule that evaluated to false, and a
// non-nil error for configuration or evaluation failures.
func ValidateTransition(schema *spec.Schema, values, oldValues any) ([]error, error) {
	var validationErrs []error

	// First we validate only object nested properties, threading the
	// corresponding old value (if any) so transition rules can fire at any depth.
	if m, ok := values.(map[string]any); ok {
		oldMap, _ := oldValues.(map[string]any)
		for propName, propSchema := range schema.Properties {
			propValue, ok := m[propName]
			if !ok {
				continue
			}
			var oldPropValue any
			if oldMap != nil {
				if v, found := oldMap[propName]; found {
					oldPropValue = v
				}
			}
			subErrs, err := ValidateTransition(&propSchema, propValue, oldPropValue)
			if err != nil {
				return nil, err
			}
			validationErrs = append(validationErrs, subErrs...)
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

	// haveOld controls transition rule evaluation at this schema level.
	// When false (no previous value at this level), rules referencing
	// oldSelf are skipped instead of being evaluated against an undefined value.
	haveOld := oldValues != nil
	celOldSelfType := cel.DynType
	var celOldSelfValue any
	if haveOld {
		celOldSelfType, celOldSelfValue, err = buildCELValueAndType(oldValues)
		if err != nil {
			return nil, err
		}
	}

	env, err := cel.NewEnv(
		cel.Variable(selfVar, celSelfType),
		cel.Variable(oldSelfVar, celOldSelfType),
	)
	if err != nil {
		return nil, fmt.Errorf("create CEL env: %w", err)
	}
	for _, r := range rules {
		ast, issues := env.Compile(r.Expression)
		if issues.Err() != nil {
			return nil, fmt.Errorf("compile the '%s' rule: %w", r.Expression, issues.Err())
		}

		// Skip transition rules when the previous value is not available
		// at this schema level (CREATE or newly added subtree).
		if !haveOld && expressionUsesOldSelf(ast) {
			continue
		}

		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("create program for the '%s' rule: %w", r.Expression, err)
		}

		out, _, err := prg.Eval(map[string]any{
			selfVar:    celSelfValue,
			oldSelfVar: celOldSelfValue,
		})
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

// expressionUsesOldSelf reports whether the compiled CEL AST references the
// "oldSelf" identifier anywhere in the expression tree. Used to detect
// transition rules so they can be skipped when no previous value is available.
func expressionUsesOldSelf(ast *cel.Ast) bool {
	if ast == nil {
		return false
	}
	rep := ast.NativeRep()
	if rep == nil {
		return false
	}
	matches := celast.MatchDescendants(celast.NavigateAST(rep), func(e celast.NavigableExpr) bool {
		return e.Kind() == celast.IdentKind && e.AsIdent() == oldSelfVar
	})
	return len(matches) > 0
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
