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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"k8s.io/utils/ptr"
)

func TestStateModuleEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state *State
		want  bool
	}{
		{name: "nil state", state: nil, want: true},
		{name: "nil module config", state: &State{}, want: true},
		{name: "enabled nil", state: &State{ModuleConfig: &cpapi.ModuleConfig{}}, want: true},
		{name: "enabled true", state: &State{ModuleConfig: &cpapi.ModuleConfig{Spec: cpapi.ModuleConfigSpec{Enabled: ptr.To(true)}}}, want: true},
		{name: "enabled false", state: &State{ModuleConfig: &cpapi.ModuleConfig{Spec: cpapi.ModuleConfigSpec{Enabled: ptr.To(false)}}}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.state.ModuleEnabled(); got != tt.want {
				t.Fatalf("ModuleEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultHelpers(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("path.one", "code_one", "message one")
	result.AddWarning("path.two", "code_two", "message two")
	result.Merge(Result{
		Errors:   []Violation{{Path: "path.three", Code: "code_three", Message: "message three", Severity: SeverityError}},
		Warnings: []Violation{{Path: "path.four", Code: "code_four", Message: "message four", Severity: SeverityWarning}},
	})

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

func TestResultErrorOrNilReturnsWrappedMessage(t *testing.T) {
	t.Parallel()

	result := Result{}
	result.AddError("Secret/x", "bad", "failed")

	err := result.ErrorOrNil()
	if err == nil || err.Error() != "Secret/x: failed" {
		t.Fatalf("ErrorOrNil() = %v, want Secret/x: failed", err)
	}
}
