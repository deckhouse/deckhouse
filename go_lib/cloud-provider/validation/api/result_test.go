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

package api

import (
	"strings"
	"testing"
)

func TestResultForNilState(t *testing.T) {
	t.Parallel()

	result := ResultForNilState()
	if !result.HasErrors() {
		t.Fatal("ResultForNilState() HasErrors() = false, want true")
	}

	violations := result.Errors()
	if len(violations) != 1 {
		t.Fatalf("ResultForNilState() errors = %d, want 1", len(violations))
	}
	if violations[0].Code != CodeInternalStateNil {
		t.Fatalf("ResultForNilState() code = %q, want %q", violations[0].Code, CodeInternalStateNil)
	}
}

func TestResultAddErrorStoresValue(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/d8-credentials.data.authScheme", "unsupported_auth_scheme", "apiToken", `authScheme "apiToken" is not allowed`)

	violations := result.Errors()
	if len(violations) != 1 {
		t.Fatalf("Errors() = %d, want 1", len(violations))
	}
	if violations[0].Value != "apiToken" {
		t.Fatalf("Errors()[0].Value = %#v, want %q", violations[0].Value, "apiToken")
	}
}

func TestResultHelpers(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("path.one", "code_one", nil, "message one")
	result.AddWarning("path.two", "code_two", nil, "message two")
	other := Result{}
	other.AddError("path.three", "code_three", nil, "message three")
	other.AddWarning("path.four", "code_four", nil, "message four")
	result.Merge(other)

	if !result.HasErrors() {
		t.Fatal("HasErrors() = false, want true")
	}
	if got := result.Error(); !strings.Contains(got, "path.one: message one") || !strings.Contains(got, "path.three: message three") {
		t.Fatalf("Error() = %q, want formatted paths", got)
	}
	if err := result.ErrorOrNil(); err == nil {
		t.Fatal("ErrorOrNil() = nil, want error")
	}

	empty := Result{}
	if empty.HasErrors() {
		t.Fatal("empty HasErrors() = true, want false")
	}
	if empty.Error() != "" {
		t.Fatalf("empty Error() = %q, want empty string", empty.Error())
	}
	if err := empty.ErrorOrNil(); err != nil {
		t.Fatalf("empty ErrorOrNil() = %v, want nil", err)
	}

	pathless := Result{}
	pathless.AddError("", "code", nil, "plain message")
	if !strings.Contains(pathless.Error(), "plain message") || strings.Contains(pathless.Error(), ": plain message") {
		t.Fatalf("Error() without path = %q", pathless.Error())
	}
}

func TestResultMergeKeepsSameCodeAtDifferentPaths(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/d8-credentials", "credential_secret_required", nil, `credential Secret "d8-credentials" is required`)

	duplicate := Result{}
	duplicate.AddError("Secret/other", "credential_secret_required", nil, "other message")
	result.Merge(duplicate)

	if len(result.Errors()) != 2 {
		t.Fatalf("Merge() errors = %d, want 2 for same code at different paths", len(result.Errors()))
	}
}

func TestResultMergeDeduplicatesViolationsByCodeAndPath(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/d8-credentials", "credential_secret_required", nil, `credential Secret "d8-credentials" is required`)

	duplicate := Result{}
	duplicate.AddError("Secret/d8-credentials", "credential_secret_required", nil, "duplicate message")
	result.Merge(duplicate)

	if len(result.Errors()) != 1 {
		t.Fatalf("Merge() errors = %d, want 1 after deduplication by code and path", len(result.Errors()))
	}

	violations := result.Errors()
	if violations[0].Code != "credential_secret_required" || violations[0].Path != "Secret/d8-credentials" {
		t.Fatalf("Errors() = %#v, want single violation for code and path pair", violations)
	}
}

func TestResultMergeKeepsDifferentCodes(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/d8-credentials", "credential_secret_required", nil, `credential Secret "d8-credentials" is required`)

	duplicate := Result{}
	duplicate.AddError("Secret/d8-credentials", "duplicate_credential_secret_required", nil, `credential Secret "d8-credentials" is required`)
	result.Merge(duplicate)

	if len(result.Errors()) != 2 {
		t.Fatalf("Merge() errors = %d, want distinct codes preserved", len(result.Errors()))
	}
}

func TestResultErrorOrNilReturnsWrappedMessage(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/x", "bad", nil, "failed")

	err := result.ErrorOrNil()
	if err == nil || err.Error() != "Secret/x: failed" {
		t.Fatalf("ErrorOrNil() = %v, want Secret/x: failed", err)
	}
}

func TestResultWarnings(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddWarning("path.a", "code_a", "val", "warning a")
	result.AddWarning("path.b", "code_b", nil, "warning b")

	warnings := result.Warnings()
	if len(warnings) != 2 {
		t.Fatalf("Warnings() = %d, want 2", len(warnings))
	}

	codes := make(map[string]bool, len(warnings))
	for _, w := range warnings {
		codes[w.Code] = true
	}
	if !codes["code_a"] {
		t.Fatal("Warnings() missing code_a")
	}
	if !codes["code_b"] {
		t.Fatal("Warnings() missing code_b")
	}
}

func TestResultMergeWarnings(t *testing.T) {
	t.Parallel()

	r1 := Result{}
	r1.AddWarning("w1", "code_w1", nil, "warn1")
	r2 := Result{}
	r2.AddWarning("w2", "code_w2", nil, "warn2")
	r1.Merge(r2)

	if len(r1.Warnings()) != 2 {
		t.Fatalf("Merged warnings = %d, want 2", len(r1.Warnings()))
	}
}

func TestResultHasErrorsAndWarnings(t *testing.T) {
	t.Parallel()

	r := Result{}
	r.AddError("e", "code_e", nil, "err")
	r.AddWarning("w", "code_w", nil, "warn")

	if !r.HasErrors() {
		t.Fatal("HasErrors() should be true with errors")
	}
	if len(r.Warnings()) != 1 {
		t.Fatalf("Warnings() = %d, want 1", len(r.Warnings()))
	}
	if len(r.Errors()) != 1 {
		t.Fatalf("Errors() = %d, want 1", len(r.Errors()))
	}
}
