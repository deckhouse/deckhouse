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

func (o *Output) AddError(path, code string, value any, message string) {
	o.errors.Add(Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	})
}

func (o *Output) AddWarning(path, code string, value any, message string) {
	o.warnings.Add(Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	})
}

func (o Output) Errors() Violations {
	return o.errors.Sorted()
}

func (o Output) Warnings() Violations {
	return o.warnings.Sorted()
}

func (o Output) HasErrors() bool {
	return len(o.errors) > 0
}

func (o Output) HasWarnings() bool {
	return len(o.warnings) > 0
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

// Sorted orders by code, then path, so the text a caller prints is stable across
// runs.
func (v Violations) Sorted() Violations {
	sorted := slices.Clone(v)

	slices.SortStableFunc(sorted, func(a, b Violation) int {
		if byCode := cmp.Compare(a.Code, b.Code); byCode != 0 {
			return byCode
		}

		return cmp.Compare(a.Path, b.Path)
	})

	return sorted
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

func (o Output) ToResponse() *protogen.ValidateResponse {
	return &protogen.ValidateResponse{
		Errors:   o.Errors().toProto(),
		Warnings: o.Warnings().toProto(),
	}
}

// OutputFromResponse rebuilds an Output, so a caller renders its text with the same
// code the validator would have used.
func OutputFromResponse(resp *protogen.ValidateResponse) Output {
	var output Output

	for _, violation := range resp.GetErrors() {
		output.AddError(
			violation.GetPath(),
			violation.GetCode(),
			valueOrNil(violation.GetValue()),
			violation.GetMessage(),
		)
	}

	for _, violation := range resp.GetWarnings() {
		output.AddWarning(
			violation.GetPath(),
			violation.GetCode(),
			valueOrNil(violation.GetValue()),
			violation.GetMessage(),
		)
	}

	return output
}

// toProto renders Value with fmt.Sprint: it is display-only, and a validator that
// must not disclose a value sends a placeholder instead of the value.
func (v Violations) toProto() []*protogen.Violation {
	if len(v) == 0 {
		return nil
	}

	wire := make([]*protogen.Violation, 0, len(v))

	for _, violation := range v {
		encoded := &protogen.Violation{
			Path:    violation.Path,
			Code:    violation.Code,
			Message: violation.Message,
		}
		if violation.Value != nil {
			encoded.Value = fmt.Sprint(violation.Value)
		}

		wire = append(wire, encoded)
	}

	return wire
}

func valueOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
