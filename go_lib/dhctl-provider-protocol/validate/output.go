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

package validate

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
)

// Output aggregates violations. The zero value is ready to use.
type Output struct {
	errors   Violations
	warnings Violations
}

func (r *Output) AddError(path, code string, value any, message string) {
	r.errors.Add(Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	})
}

func (r *Output) AddWarning(path, code string, value any, message string) {
	r.warnings.Add(Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	})
}

func (r Output) Errors() Violations {
	return r.errors.Sorted()
}

func (r Output) Warnings() Violations {
	return r.warnings.Sorted()
}

func (r Output) HasErrors() bool {
	return len(r.errors) > 0
}

func (r Output) HasWarnings() bool {
	return len(r.warnings) > 0
}

type Violation struct {
	Path    string `json:"path,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

type Violations []Violation

func (v *Violations) Add(violation Violation) {
	*v = append(*v, violation)
}

func (v Violations) Sorted() Violations {
	ret := slices.Clone(v)
	slices.SortStableFunc(ret, func(a, b Violation) int {
		if by := cmp.Compare(a.Code, b.Code); by != 0 {
			return by
		}
		return cmp.Compare(a.Path, b.Path)
	})
	return ret
}

func (v Violations) String() string {
	if len(v) == 0 {
		return ""
	}

	var builder strings.Builder

	for i, violation := range v.Sorted() {
		if i > 0 {
			builder.WriteByte('\n')
		}
		if violation.Path == "" {
			builder.WriteString(violation.Message)
		} else {
			builder.WriteString(violation.Path)
			builder.WriteString(": ")
			builder.WriteString(violation.Message)
		}
	}
	return builder.String()
}

func ToPBResponse(result Output) *protogen.ValidateResponse {
	return &protogen.ValidateResponse{
		Errors:   toPBViolations(result.Errors()),
		Warnings: toPBViolations(result.Warnings()),
	}
}

// FromPBResponse rebuilds a Result, so a host renders its text with the same code
// the plugin would have used.
func FromPBResponse(resp *protogen.ValidateResponse) Output {
	var result Output

	for _, violation := range resp.GetErrors() {
		result.AddError(
			violation.GetPath(),
			violation.GetCode(),
			valueOrNil(violation.GetValue()),
			violation.GetMessage(),
		)
	}

	for _, violation := range resp.GetWarnings() {
		result.AddWarning(
			violation.GetPath(),
			violation.GetCode(),
			valueOrNil(violation.GetValue()),
			violation.GetMessage(),
		)
	}

	return result
}

// toPBViolations renders Value with fmt.Sprint: it is display-only, and a plugin
// that must not disclose a value sends a placeholder instead of the value.
func toPBViolations(violations Violations) []*protogen.Violation {
	if len(violations) == 0 {
		return nil
	}

	ret := make([]*protogen.Violation, 0, len(violations))

	for _, violation := range violations {
		wire := &protogen.Violation{
			Path:    violation.Path,
			Code:    violation.Code,
			Message: violation.Message,
		}
		if violation.Value != nil {
			wire.Value = fmt.Sprint(violation.Value)
		}

		ret = append(ret, wire)
	}

	return ret
}

func valueOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
