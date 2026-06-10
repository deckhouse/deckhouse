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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// State holds decoded provider resources used by validation rules.
type State struct {
	ModuleConfig      *cpapi.ModuleConfig
	CredentialSecrets []cpapi.CredentialSecret
	NodeGroups        []cpapi.NodeGroup
	InstanceClasses   []cpapi.InstanceClass

	// MigrationStatus controls whether new-model validation should run.
	MigrationStatus cpapi.MigrationStatus

	// LegacyProviderClusterConfig holds the legacy providerClusterConfiguration section.
	LegacyProviderClusterConfig map[string]any
}

// ModuleEnabled reports whether the cloud-provider module is enabled in the current state.
func (s *State) ModuleEnabled() bool {
	if s == nil || s.ModuleConfig == nil || s.ModuleConfig.Spec.Enabled == nil {
		return true
	}

	return *s.ModuleConfig.Spec.Enabled
}

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
	Errors   []Violation `json:"errors,omitempty"`
	Warnings []Violation `json:"warnings,omitempty"`
}

// AddError appends a blocking validation violation.
func (r *Result) AddError(path, code, message string) {
	r.Errors = append(r.Errors, Violation{
		Path:     path,
		Code:     code,
		Message:  message,
		Severity: SeverityError,
	})
}

// AddWarning appends a non-blocking validation violation.
func (r *Result) AddWarning(path, code, message string) {
	r.Warnings = append(r.Warnings, Violation{
		Path:     path,
		Code:     code,
		Message:  message,
		Severity: SeverityWarning,
	})
}

// Merge appends errors and warnings from another result.
func (r *Result) Merge(other Result) {
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

// HasErrors reports whether the result contains blocking violations.
func (r Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// Error returns a human-readable summary of blocking violations.
func (r Result) Error() string {
	if !r.HasErrors() {
		return ""
	}

	lines := make([]string, 0, len(r.Errors))
	for _, violation := range r.Errors {
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
