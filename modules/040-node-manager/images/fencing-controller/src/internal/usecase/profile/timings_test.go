/*
Copyright 2026 Flant JSC

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

package profile

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fencing-controller/internal/domain/fsm"
)

func TestTimingsTakesOnlyWhatTheControllerActsOn(t *testing.T) {
	profile := profileWith("critical", fsm.Params{FallbackTTL: time.Second, EvacuationDelay: 1200 * time.Millisecond})
	// Values that belong to the agent must not leak into the decision inputs.
	profile.Spec.Memberlist.ProbeInterval = metav1.Duration{Duration: 100 * time.Millisecond}
	profile.Spec.Watchdog.Timeout = metav1.Duration{Duration: 3 * time.Second}

	got, err := timings(profile)
	if err != nil {
		t.Fatalf("read timings: %v", err)
	}

	want := fsm.Params{FallbackTTL: time.Second, EvacuationDelay: 1200 * time.Millisecond}
	if got != want {
		t.Errorf("read %+v, want %+v", got, want)
	}
}

func TestTimingsRejectsValuesTheControllerCannotActOn(t *testing.T) {
	for name, tc := range map[string]struct {
		params  fsm.Params
		wantSub string
	}{
		"no fallback ttl": {
			params:  fsm.Params{EvacuationDelay: time.Second},
			wantSub: "fallback.ttl",
		},
		"negative fallback ttl": {
			params:  fsm.Params{FallbackTTL: -time.Second, EvacuationDelay: time.Second},
			wantSub: "fallback.ttl",
		},
		"no evacuation delay": {
			params:  fsm.Params{FallbackTTL: time.Second},
			wantSub: "evacuation.delay",
		},
		"negative evacuation delay": {
			params:  fsm.Params{FallbackTTL: time.Second, EvacuationDelay: -time.Second},
			wantSub: "evacuation.delay",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := timings(profileWith("critical", tc.params))
			if err == nil {
				t.Fatal("read succeeded, want an error so the incident is reported as misconfigured")
			}

			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not mention %s", err, tc.wantSub)
			}
		})
	}
}

// TestTimingsReportsEveryViolationAtOnce lets an operator fix a broken profile in
// one pass instead of one field per reconcile.
func TestTimingsReportsEveryViolationAtOnce(t *testing.T) {
	_, err := timings(profileWith("critical", fsm.Params{}))
	if err == nil {
		t.Fatal("read of an empty profile succeeded, want an error")
	}

	for _, field := range []string{"fallback.ttl", "evacuation.delay"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error %q does not mention %s", err, field)
		}
	}
}

func TestTimingsRejectsNoProfile(t *testing.T) {
	if _, err := timings(nil); err == nil {
		t.Error("read of a nil profile succeeded, want an error")
	}
}
