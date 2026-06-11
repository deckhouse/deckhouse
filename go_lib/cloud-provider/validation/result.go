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

// Package validation provides shared validation helpers for cloud-provider modules.
package validation

import (
	"errors"
	"strings"

	"golang.org/x/exp/maps"
)

const (
	// SeverityError marks a validation violation that must block the operation.
	SeverityError = "error"
	// SeverityWarning marks a non-blocking validation violation.
	SeverityWarning = "warning"
)

// Violation describes a single validation problem with a machine-readable code.
type Violation struct {
	// Path is the resource field path, such as Secret/d8-credentials.data.secret.
	Path string `json:"path,omitempty"`
	// Code is a stable machine-readable violation identifier.
	Code string `json:"code,omitempty"`
	// Message is a human-readable explanation of the violation.
	Message string `json:"message"`
	// Severity is either SeverityError or SeverityWarning.
	Severity string `json:"severity"`
}

// Result aggregates validation errors and warnings.
type Result struct {
	errors   map[string]Violation
	warnings map[string]Violation
}

// AddError records a blocking validation violation.
func (r *Result) AddError(path, code, message string) {
	if r.errors == nil {
		r.errors = make(map[string]Violation)
	}

	r.errors[violationKey(code, path)] = Violation{
		Path:     path,
		Code:     code,
		Message:  message,
		Severity: SeverityError,
	}
}

// AddWarning records a non-blocking validation violation.
func (r *Result) AddWarning(path, code, message string) {
	if r.warnings == nil {
		r.warnings = make(map[string]Violation)
	}

	r.warnings[violationKey(code, path)] = Violation{
		Path:     path,
		Code:     code,
		Message:  message,
		Severity: SeverityWarning,
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
		for key, violation := range result.errors {
			r.errors[key] = violation
		}

		for key, violation := range result.warnings {
			r.warnings[key] = violation
		}
	}
}

// Errors returns blocking violations.
func (r Result) Errors() []Violation {
	return maps.Values(r.errors)
}

// Warnings returns non-blocking violations.
func (r Result) Warnings() []Violation {
	return maps.Values(r.warnings)
}

// HasErrors reports whether the result contains blocking violations.
func (r Result) HasErrors() bool {
	return len(r.errors) > 0
}

// Error returns a human-readable summary of blocking violations.
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

		lines = append(lines, violation.Path+": "+violation.Message)
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
