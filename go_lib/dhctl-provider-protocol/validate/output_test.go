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
	"reflect"
	"testing"

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
)

func TestOutputAddError(t *testing.T) {
	tests := []struct {
		name       string
		violations Violations
		want       Violations
	}{
		{name: "records nothing by default"},
		{
			name:       "records a violation",
			violations: Violations{{Path: "Secret/d8-credentials", Code: "secret_required", Message: "required", Value: "masked"}},
			want:       Violations{{Path: "Secret/d8-credentials", Code: "secret_required", Message: "required", Value: "masked"}},
		},
		{
			name: "orders what it recorded",
			violations: Violations{
				{Path: "z", Code: "b_code", Message: "second"},
				{Path: "a", Code: "a_code", Message: "first"},
			},
			want: Violations{
				{Path: "a", Code: "a_code", Message: "first"},
				{Path: "z", Code: "b_code", Message: "second"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output Output
			for _, violation := range test.violations {
				output.AddError(violation.Path, violation.Code, violation.Value, violation.Message)
			}

			if got := output.Errors(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("Errors() = %+v, want %+v", got, test.want)
			}

			if got, want := output.HasErrors(), len(test.want) > 0; got != want {
				t.Errorf("HasErrors() = %v, want %v", got, want)
			}

			// Errors and warnings are separate ledgers.
			if output.HasWarnings() {
				t.Error("HasWarnings() = true, want false")
			}
		})
	}
}

func TestOutputAddWarning(t *testing.T) {
	tests := []struct {
		name       string
		violations Violations
		want       Violations
	}{
		{name: "records nothing by default"},
		{
			name:       "records a warning",
			violations: Violations{{Path: "NodeGroup/worker", Code: "replicas_zero", Message: "replicas is 0"}},
			want:       Violations{{Path: "NodeGroup/worker", Code: "replicas_zero", Message: "replicas is 0"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output Output
			for _, violation := range test.violations {
				output.AddWarning(violation.Path, violation.Code, violation.Value, violation.Message)
			}

			if got := output.Warnings(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("Warnings() = %+v, want %+v", got, test.want)
			}

			if got, want := output.HasWarnings(), len(test.want) > 0; got != want {
				t.Errorf("HasWarnings() = %v, want %v", got, want)
			}

			// A warning never makes a configuration invalid.
			if output.HasErrors() {
				t.Error("HasErrors() = true with only warnings")
			}
		})
	}
}

func TestViolationsSorted(t *testing.T) {
	tests := []struct {
		name       string
		violations Violations
		want       Violations
	}{
		{name: "handles the empty set", violations: Violations{}, want: Violations{}},
		{
			name: "orders by code, then path",
			violations: Violations{
				{Path: "z", Code: "b_code"},
				{Path: "b", Code: "a_code"},
				{Path: "a", Code: "a_code"},
			},
			want: Violations{
				{Path: "a", Code: "a_code"},
				{Path: "b", Code: "a_code"},
				{Path: "z", Code: "b_code"},
			},
		},
		{
			name: "keeps repeats in the order they were added",
			violations: Violations{
				{Path: "a", Code: "a_code", Message: "first"},
				{Path: "a", Code: "a_code", Message: "second"},
			},
			want: Violations{
				{Path: "a", Code: "a_code", Message: "first"},
				{Path: "a", Code: "a_code", Message: "second"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := append(Violations{}, test.violations...)

			if got := test.violations.Sorted(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("Sorted() = %+v, want %+v", got, test.want)
			}

			// Sorting must not reorder the caller's own slice.
			if !reflect.DeepEqual(test.violations, source) {
				t.Errorf("receiver = %+v, want it untouched: %+v", test.violations, source)
			}
		})
	}
}

func TestViolationsString(t *testing.T) {
	tests := []struct {
		name       string
		violations Violations
		want       string
	}{
		{name: "renders the empty set as empty text"},
		{
			name:       "renders path and message",
			violations: Violations{{Path: "NodeGroup/master", Code: "required", Message: "is required"}},
			want:       "NodeGroup/master: is required",
		},
		{
			name:       "drops an empty path",
			violations: Violations{{Code: "internal", Message: "state is not initialized"}},
			want:       "state is not initialized",
		},
		{
			// The caller prints this verbatim and validators compare whole strings,
			// so the order is part of the contract.
			name: "renders one line per violation in order",
			violations: Violations{
				{Path: "z", Code: "b_code", Message: "second"},
				{Path: "a", Code: "a_code", Message: "first"},
			},
			want: "a: first\nz: second",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.violations.String(); got != test.want {
				t.Errorf("String() =\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

func TestOutputToResponse(t *testing.T) {
	tests := []struct {
		name         string
		errors       Violations
		warnings     Violations
		wantErrors   []*protogen.Violation
		wantWarnings []*protogen.Violation
	}{
		{name: "sends nothing when the configuration is valid"},
		{
			name:         "splits errors from warnings",
			errors:       Violations{{Path: "a", Code: "a_code", Message: "blocking"}},
			warnings:     Violations{{Path: "b", Code: "b_code", Message: "advisory"}},
			wantErrors:   []*protogen.Violation{{Path: "a", Code: "a_code", Message: "blocking"}},
			wantWarnings: []*protogen.Violation{{Path: "b", Code: "b_code", Message: "advisory"}},
		},
		{
			// A non-string Value is rendered, not dropped: rules report rejected
			// values of any type and the wire form is display-only.
			name:       "renders a non-string value",
			errors:     Violations{{Path: "DVPInstanceClass/worker.spec.etcdDisk", Code: "forbidden", Message: "not allowed", Value: 3}},
			wantErrors: []*protogen.Violation{{Path: "DVPInstanceClass/worker.spec.etcdDisk", Code: "forbidden", Message: "not allowed", Value: "3"}},
		},
		{
			name:       "leaves an absent value empty",
			errors:     Violations{{Path: "a", Code: "a_code", Message: "blocking"}},
			wantErrors: []*protogen.Violation{{Path: "a", Code: "a_code", Message: "blocking"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output Output
			for _, violation := range test.errors {
				output.AddError(violation.Path, violation.Code, violation.Value, violation.Message)
			}

			for _, violation := range test.warnings {
				output.AddWarning(violation.Path, violation.Code, violation.Value, violation.Message)
			}

			resp := output.ToResponse()

			if got := violationsOf(resp.GetErrors()); !reflect.DeepEqual(got, violationsOf(test.wantErrors)) {
				t.Errorf("Errors = %+v, want %+v", got, violationsOf(test.wantErrors))
			}

			if got := violationsOf(resp.GetWarnings()); !reflect.DeepEqual(got, violationsOf(test.wantWarnings)) {
				t.Errorf("Warnings = %+v, want %+v", got, violationsOf(test.wantWarnings))
			}
		})
	}
}

func TestOutputFromResponse(t *testing.T) {
	tests := []struct {
		name         string
		resp         *protogen.ValidateResponse
		wantErrors   Violations
		wantWarnings Violations
	}{
		{
			// Fail open is not an option, but neither is inventing a violation: no
			// response means nothing was reported.
			name: "reads a missing response as valid",
		},
		{
			name: "reads errors and warnings",
			resp: &protogen.ValidateResponse{
				Errors:   []*protogen.Violation{{Path: "a", Code: "a_code", Message: "blocking", Value: "masked"}},
				Warnings: []*protogen.Violation{{Path: "b", Code: "b_code", Message: "advisory"}},
			},
			wantErrors:   Violations{{Path: "a", Code: "a_code", Message: "blocking", Value: "masked"}},
			wantWarnings: Violations{{Path: "b", Code: "b_code", Message: "advisory"}},
		},
		{
			// An empty wire value means "no value", not the empty string.
			name: "reads an empty value as absent",
			resp: &protogen.ValidateResponse{
				Errors: []*protogen.Violation{{Path: "a", Code: "a_code", Message: "blocking", Value: ""}},
			},
			wantErrors: Violations{{Path: "a", Code: "a_code", Message: "blocking"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := OutputFromResponse(test.resp)

			if got := output.Errors(); !reflect.DeepEqual(got, test.wantErrors.Sorted()) {
				t.Errorf("Errors() = %+v, want %+v", got, test.wantErrors)
			}

			if got := output.Warnings(); !reflect.DeepEqual(got, test.wantWarnings.Sorted()) {
				t.Errorf("Warnings() = %+v, want %+v", got, test.wantWarnings)
			}
		})
	}
}

// violationsOf compares wire violations by value instead of by protobuf internals.
func violationsOf(wire []*protogen.Violation) Violations {
	violations := make(Violations, 0, len(wire))
	for _, violation := range wire {
		violations = append(violations, Violation{
			Path:    violation.GetPath(),
			Code:    violation.GetCode(),
			Message: violation.GetMessage(),
			Value:   valueOrNil(violation.GetValue()),
		})
	}

	return violations
}
