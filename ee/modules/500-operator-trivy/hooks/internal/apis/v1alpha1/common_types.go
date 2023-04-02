/*
Copyright 2023 Flant JSC

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

// https://github.com/aquasecurity/trivy-operator/blob/84df941b628441c285c08850bf73fd0e5fd3aa05/pkg/apis/aquasecurity/v1alpha1/common_types.go

package v1alpha1

import (
	"fmt"
	"strings"
)

const (
	TTLReportAnnotation = "trivy-operator.aquasecurity.github.io/report-ttl"
)

// Severity level of a vulnerability or a configuration audit check.
// +enum
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"

	SeverityUnknown Severity = "UNKNOWN"
)

// StringToSeverity returns the enum constant of Severity with the specified
// name. The name must match exactly an identifier used to declare an enum
// constant. (Extraneous whitespace characters are not permitted.)
func StringToSeverity(name string) (Severity, error) {
	s := strings.ToUpper(name)
	switch s {
	case "CRITICAL", "HIGH", "MEDIUM", "LOW", "NONE", "UNKNOWN":
		return Severity(s), nil
	default:
		return "", fmt.Errorf("unrecognized name literal: %s", name)
	}
}

const ScannerNameTrivy = "Trivy"

// Scanner is the spec for a scanner generating a security assessment report.
type Scanner struct {
	// Name the name of the scanner.
	Name string `json:"name"`

	// Vendor the name of the vendor providing the scanner.
	Vendor string `json:"vendor"`

	// Version the version of the scanner.
	Version string `json:"version"`
}
