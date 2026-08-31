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

package v1

import (
	"fmt"
	"strings"
)

// Output aggregates violations. The zero value is ready to use.
type Output struct {
	Errors   Violations `json:"errors,omitempty"`
	Warnings Violations `json:"warnings,omitempty"`
}

type Violations []Violation

type Violation struct {
	Path    string `json:"path,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Value   any    `json:"value"`
}

func (o *Output) AddError(path, code string, value any, message string) {
	o.Errors.Add(Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	})
}

func (o *Output) AddWarning(path, code string, value any, message string) {
	o.Warnings.Add(Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	})
}

func (o Output) ToResponse() *ValidateResponse {
	return &ValidateResponse{
		Errors:   o.Errors.ToResponse(),
		Warnings: o.Warnings.ToResponse(),
	}
}

func (v *Violations) Add(violation Violation) {
	*v = append(*v, violation)
}

func (v Violations) String() string {
	if len(v) == 0 {
		return ""
	}

	var builder strings.Builder

	for i, violation := range v {
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

// ToResponse renders Value with fmt.Sprint: it is display-only, and a validator that
// must not disclose a value sends a placeholder instead of the value.
func (v Violations) ToResponse() []*ViolationResponse {
	if len(v) == 0 {
		return nil
	}

	wire := make([]*ViolationResponse, 0, len(v))

	for _, violation := range v {
		encoded := &ViolationResponse{
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

// OutputFromResponse rebuilds an Output, so a caller renders its text with the same
// code the validator would have used.
func OutputFromResponse(resp *ValidateResponse) Output {
	var ret Output

	for _, violation := range resp.GetErrors() {
		ret.AddError(
			violation.GetPath(),
			violation.GetCode(),
			valueOrNil(violation.GetValue()),
			violation.GetMessage(),
		)
	}

	for _, violation := range resp.GetWarnings() {
		ret.AddWarning(
			violation.GetPath(),
			violation.GetCode(),
			valueOrNil(violation.GetValue()),
			violation.GetMessage(),
		)
	}

	return ret
}

func valueOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
