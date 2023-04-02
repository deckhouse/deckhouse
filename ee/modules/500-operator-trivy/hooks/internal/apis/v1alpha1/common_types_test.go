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

// https://github.com/aquasecurity/trivy-operator/blob/84df941b628441c285c08850bf73fd0e5fd3aa05/pkg/apis/aquasecurity/v1alpha1/common_types_test.go

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/ee/modules/500-operator-trivy/hooks/internal/apis/v1alpha1"
)

func TestStringToSeverity(t *testing.T) {
	testCases := []struct {
		name             string
		expectedSeverity v1alpha1.Severity
		expectedError    string
	}{
		{
			name:          "xxx",
			expectedError: "unrecognized name literal: xxx",
		},
		{
			name:             "CRITICAL",
			expectedSeverity: v1alpha1.SeverityCritical,
		},
		{
			name:             "HIGH",
			expectedSeverity: v1alpha1.SeverityHigh,
		},
		{
			name:             "MEDIUM",
			expectedSeverity: v1alpha1.SeverityMedium,
		},
		{
			name:             "LOW",
			expectedSeverity: v1alpha1.SeverityLow,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			severity, err := v1alpha1.StringToSeverity(tc.name)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedSeverity, severity)
			}
		})
	}
}
