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

package observed

import (
	"encoding/json"
	"testing"
)

// A hook failure lasts until that hook succeeds or the package's hooks stop; a retry that fails the
// same way changes nothing, and a write that changes nothing leaves no entry behind.
func TestHookFailure(t *testing.T) {
	notified := 0
	s := New(func(string) { notified++ })

	s.ClearHookFailure("p", "h")
	if len(s.Names()) != 0 || notified != 0 {
		t.Fatalf("a clear without a failure created an entry or notified: names %v, notified %d", s.Names(), notified)
	}

	s.RecordHookFailure("p", "h", HookReasonEvent, "boom")
	since := s.Get("p").HookFailure.Since

	s.RecordHookFailure("p", "h", HookReasonEvent, "boom")
	if notified != 1 || s.Get("p").HookFailure.Since != since {
		t.Fatalf("a repeated failure notified or moved Since: notified %d", notified)
	}

	s.ClearHookFailure("p", "other")
	if s.Get("p").HookFailure == nil {
		t.Fatal("the success of another hook cleared the failure")
	}

	s.ClearHookFailure("p", "h")
	if s.Get("p").HookFailure != nil || notified != 2 {
		t.Fatalf("the success of the failed hook did not clear it: notified %d", notified)
	}

	s.RecordHookFailure("p", "sync", HookReasonSync, "x")
	s.Get("p").HookFailure.Message = "changed by the caller"
	if got := s.Get("p").HookFailure.Message; got != "x" {
		t.Fatalf("Get returned the stored failure, not a copy: %q", got)
	}

	s.ResetHookFailure("p")
	if s.Get("p").HookFailure != nil {
		t.Fatal("stopping the hooks did not clear the failure")
	}
}

// The tracking marshals to status.tracking as the external contract has it: completed, remaining
// and nelm's report unchanged.
func TestTrackingJSON(t *testing.T) {
	s := New(nil)
	report := json.RawMessage(`{"operations":[{"id":"a","status":"Completed"}]}`)
	s.UpdateTracking("p", Tracking{Completed: 1, Remaining: 2, Report: report})

	raw, err := json.Marshal(s.Get("p").Tracking)
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"completed":1,"remaining":2,"report":{"operations":[{"id":"a","status":"Completed"}]}}`; string(raw) != want {
		t.Fatalf("status.tracking %s, want %s", raw, want)
	}

	s.Get("p").Tracking.Report[0] = 'x'
	if s.Get("p").Tracking.Report[0] != '{' {
		t.Fatal("Get returned the stored report, not a copy")
	}

	s.UpdateTracking("p", Tracking{Remaining: 1, Report: report})

	s.ResetTracking("p")
	if !s.Get("p").Tracking.Empty() {
		t.Fatal("reset left the progress")
	}

	s.Forget("p")
	if len(s.Names()) != 0 {
		t.Fatalf("Forget left %v", s.Names())
	}
}
