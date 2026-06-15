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

package validation

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

func TestResultHelpers(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("path.one", "code_one", "message one")
	result.AddWarning("path.two", "code_two", "message two")
	other := Result{}
	other.AddError("path.three", "code_three", "message three")
	other.AddWarning("path.four", "code_four", "message four")
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
	pathless.AddError("", "code", "plain message")
	if !strings.Contains(pathless.Error(), "plain message") || strings.Contains(pathless.Error(), ": plain message") {
		t.Fatalf("Error() without path = %q", pathless.Error())
	}
}

func TestResultMergeKeepsSameCodeAtDifferentPaths(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/d8-credentials", "credential_secret_required", `credential Secret "d8-credentials" is required`)

	duplicate := Result{}
	duplicate.AddError("Secret/other", "credential_secret_required", "other message")
	result.Merge(duplicate)

	if len(result.Errors()) != 2 {
		t.Fatalf("Merge() errors = %d, want 2 for same code at different paths", len(result.Errors()))
	}
}

func TestResultMergeDeduplicatesViolationsByCodeAndPath(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/d8-credentials", "credential_secret_required", `credential Secret "d8-credentials" is required`)

	duplicate := Result{}
	duplicate.AddError("Secret/d8-credentials", "credential_secret_required", "duplicate message")
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
	result.AddError("Secret/d8-credentials", "credential_secret_required", `credential Secret "d8-credentials" is required`)

	duplicate := Result{}
	duplicate.AddError("Secret/d8-credentials", "duplicate_credential_secret_required", `credential Secret "d8-credentials" is required`)
	result.Merge(duplicate)

	if len(result.Errors()) != 2 {
		t.Fatalf("Merge() errors = %d, want distinct codes preserved", len(result.Errors()))
	}
}

func TestResultErrorOrNilReturnsWrappedMessage(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/x", "bad", "failed")

	err := result.ErrorOrNil()
	if err == nil || err.Error() != "Secret/x: failed" {
		t.Fatalf("ErrorOrNil() = %v, want Secret/x: failed", err)
	}
}
