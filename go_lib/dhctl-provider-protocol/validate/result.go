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
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Result aggregates validation errors and warnings, deduplicated by code+path.
// The zero value is ready to use.
type Result struct {
	errors   map[string]Violation
	warnings map[string]Violation
}

// Violation describes a single validation problem with a machine-readable code.
type Violation struct {
	// Path is the resource field path, such as Secret/d8-credentials.data.secret.
	Path string `json:"path,omitempty"`
	// Code is a stable machine-readable violation identifier.
	Code string `json:"code,omitempty"`
	// Message is a human-readable explanation of the violation.
	Message string `json:"message"`
	// Value is the rejected field value. Kept as any because in-cluster admission
	// paths feed it into field.Invalid; the wire format renders it with fmt.Sprint.
	Value any `json:"value,omitempty"`
}

// NewResult builds a Result from violation slices. Use it when the violations come
// off the wire rather than from AddError/AddWarning calls.
func NewResult(errs, warns []Violation) Result {
	result := Result{}
	for _, violation := range errs {
		result.AddError(violation.Path, violation.Code, violation.Value, violation.Message)
	}
	for _, violation := range warns {
		result.AddWarning(violation.Path, violation.Code, violation.Value, violation.Message)
	}

	return result
}

// AddError records a blocking validation violation. value is the rejected field
// value shown to the user.
func (r *Result) AddError(path, code string, value any, message string) {
	if r.errors == nil {
		r.errors = make(map[string]Violation)
	}

	r.errors[violationKey(code, path)] = Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	}
}

// AddWarning records a non-blocking validation violation.
func (r *Result) AddWarning(path, code string, value any, message string) {
	if r.warnings == nil {
		r.warnings = make(map[string]Violation)
	}

	r.warnings[violationKey(code, path)] = Violation{
		Path:    path,
		Code:    code,
		Message: message,
		Value:   value,
	}
}

// Merge copies unique violations from another result.
func (r *Result) Merge(results ...Result) {
	if r.errors == nil {
		r.errors = make(map[string]Violation)
	}
	if r.warnings == nil {
		r.warnings = make(map[string]Violation)
	}

	for _, result := range results {
		maps.Copy(r.errors, result.errors)
		maps.Copy(r.warnings, result.warnings)
	}
}

// Errors returns blocking violations ordered by code and path.
func (r Result) Errors() []Violation {
	return sortedViolations(r.errors)
}

// Warnings returns non-blocking violations ordered by code and path.
func (r Result) Warnings() []Violation {
	return sortedViolations(r.warnings)
}

// HasErrors reports whether the result contains blocking violations.
func (r Result) HasErrors() bool {
	return len(r.errors) > 0
}

// Error returns a human-readable summary of blocking violations. Both the plugin
// and the host format their user-facing text with this method, so the text stays
// identical no matter which side renders it.
//
// Result is not an error: an empty Result means "valid", and a type whose Error()
// method makes it satisfy the error interface would read as a failure the moment it
// is returned as one. Use ErrorOrNil where an error is wanted.
func (r Result) Error() string {
	if !r.HasErrors() {
		return ""
	}

	lines := make([]string, 0, len(r.errors))
	for _, violation := range r.Errors() {
		if violation.Path == "" {
			lines = append(lines, violation.Message)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", violation.Path, violation.Message))
	}

	return strings.Join(lines, "\n")
}

// ErrorOrNil returns nil when there are no blocking violations.
func (r Result) ErrorOrNil() error {
	if !r.HasErrors() {
		return nil
	}

	return errors.New(r.Error())
}

func violationKey(code, path string) string {
	return code + "\x00" + path
}

func sortedViolations(violations map[string]Violation) []Violation {
	result := make([]Violation, 0, len(violations))
	for _, key := range slices.Sorted(maps.Keys(violations)) {
		result = append(result, violations[key])
	}

	return result
}
