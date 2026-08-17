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

import "testing"

func TestResultZeroValueIsUsable(t *testing.T) {
	var result Result

	if result.HasErrors() {
		t.Error("HasErrors() = true on a zero Result")
	}
	if err := result.ErrorOrNil(); err != nil {
		t.Errorf("ErrorOrNil() = %v, want nil", err)
	}
	if got := result.Error(); got != "" {
		t.Errorf("Error() = %q, want empty", got)
	}
}

// Violations are keyed by code+path, so a rule reached from two directions reports
// once. The host prints this text verbatim; a duplicated line reads as two problems.
func TestResultDeduplicatesByCodeAndPath(t *testing.T) {
	var result Result
	result.AddError("Secret/d8-credentials", "credential_secret_required", nil, "credential Secret is required")
	result.AddError("Secret/d8-credentials", "credential_secret_required", nil, "credential Secret is required")

	if got := len(result.Errors()); got != 1 {
		t.Fatalf("Errors() = %d violations, want 1", got)
	}
}

// The order must not depend on map iteration: the host's output and the exact-string
// tests of a plugin both compare whole texts.
func TestResultOrdersViolationsByCodeThenPath(t *testing.T) {
	var result Result
	result.AddError("z", "b_code", nil, "second")
	result.AddError("a", "a_code", nil, "first")
	result.AddError("b", "a_code", nil, "middle")

	want := "a: first\nb: middle\nz: second"
	if got := result.Error(); got != want {
		t.Fatalf("Error() =\n%s\nwant\n%s", got, want)
	}
}

func TestResultErrorOmitsEmptyPath(t *testing.T) {
	var result Result
	result.AddError("", "internal", nil, "state is not initialized")

	if got := result.Error(); got != "state is not initialized" {
		t.Fatalf("Error() = %q", got)
	}
}

// Warnings never turn a valid configuration into an invalid one.
func TestResultWarningsAreNotBlocking(t *testing.T) {
	var result Result
	result.AddWarning("NodeGroup/worker", "replicas_zero", nil, "replicas is 0")

	if result.HasErrors() {
		t.Error("HasErrors() = true with only warnings")
	}
	if err := result.ErrorOrNil(); err != nil {
		t.Errorf("ErrorOrNil() = %v, want nil", err)
	}
	if got := len(result.Warnings()); got != 1 {
		t.Errorf("Warnings() = %d, want 1", got)
	}
}

func TestResultMergeKeepsBothKinds(t *testing.T) {
	var first Result
	first.AddError("a", "a_code", nil, "error a")

	var second Result
	second.AddError("b", "b_code", nil, "error b")
	second.AddWarning("c", "c_code", nil, "warning c")

	first.Merge(second)

	if got := len(first.Errors()); got != 2 {
		t.Errorf("Errors() = %d, want 2", got)
	}
	if got := len(first.Warnings()); got != 1 {
		t.Errorf("Warnings() = %d, want 1", got)
	}
}

func TestNewResultRebuildsFromViolations(t *testing.T) {
	result := NewResult(
		[]Violation{{Path: "a", Code: "a_code", Message: "error a", Value: "masked"}},
		[]Violation{{Path: "b", Code: "b_code", Message: "warning b"}},
	)

	if got := result.Error(); got != "a: error a" {
		t.Errorf("Error() = %q", got)
	}
	if got := result.Errors()[0].Value; got != "masked" {
		t.Errorf("Value = %v, want %q", got, "masked")
	}
	if got := len(result.Warnings()); got != 1 {
		t.Errorf("Warnings() = %d, want 1", got)
	}
}
