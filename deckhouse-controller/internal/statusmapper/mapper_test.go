// Copyright 2025 Flant JSC
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

package statusmapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectDuplicateCases(t *testing.T) {
	testSpecs := []Spec{
		{
			Type: "Test",
			Rule: FirstMatch{
				{When: IsTrue("A"), Status: metav1.ConditionTrue},
				{When: IsFalse("A"), Status: metav1.ConditionFalse},
				{Status: metav1.ConditionFalse}, // default fallback
			},
		},
	}

	mapper := NewMapper(testSpecs)
	warnings := mapper.DetectDuplicateCases()

	// No warnings expected for well-formed specs
	assert.Empty(t, warnings, "well-formed specs should have no duplicate cases")
}

func TestDetectDuplicateCases_WithDuplicates(t *testing.T) {
	testSpecs := []Spec{
		{
			Type: "Test",
			Rule: FirstMatch{
				{When: Always{}, Status: metav1.ConditionTrue},
				{When: Always{}, Status: metav1.ConditionFalse}, // shadowed
			},
		},
	}

	mapper := NewMapper(testSpecs)
	warnings := mapper.DetectDuplicateCases()
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "shadowed")
}

func TestDetectDuplicateCases_DefaultShadows(t *testing.T) {
	testSpecs := []Spec{
		{
			Type: "Test",
			Rule: FirstMatch{
				{When: IsTrue("A"), Status: metav1.ConditionTrue},
				{Status: metav1.ConditionFalse},                     // default case
				{When: IsFalse("A"), Status: metav1.ConditionFalse}, // shadowed by default
			},
		},
	}

	mapper := NewMapper(testSpecs)
	warnings := mapper.DetectDuplicateCases()
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "default case")
}

func TestMatcher_String(t *testing.T) {
	tests := []struct {
		name     string
		matcher  Matcher
		expected string
	}{
		{"Always", Always{}, "Always"},
		{"ConditionIs", ConditionIs{Name: "Test", Status: metav1.ConditionTrue}, "Test=True"},
		{"ConditionNotTrue", ConditionNotTrue{Name: "Test"}, "Test!=True"},
		{"AllOf", AllOf{ConditionTrue("A"), ConditionTrue("B")}, "AllOf(A=True AND B=True)"},
		{"AnyOf", AnyOf{ConditionTrue("A"), ConditionTrue("B")}, "AnyOf(A=True OR B=True)"},
		{"Predicate", Predicate{Name: "custom"}, "Predicate(custom)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.matcher.String())
		})
	}
}
