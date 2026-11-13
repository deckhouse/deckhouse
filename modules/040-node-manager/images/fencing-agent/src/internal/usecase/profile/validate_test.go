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
	"errors"
	"strings"
	"testing"
	"time"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func TestValidateRejectsNilProfile(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("Validate accepted a nil profile")
	}
}

// Each case breaks one rule of a valid profile. The error must name the rule.
func TestValidateRejectsBrokenProfiles(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(s *v1alpha1.FencingSLAProfileSpec)
		want   string
	}{
		{"zero duration", func(s *v1alpha1.FencingSLAProfileSpec) { s.Memberlist.ProbeInterval = dur(0) }, "memberlist.probeInterval: must be positive"},
		{"negative duration", func(s *v1alpha1.FencingSLAProfileSpec) { s.Watchdog.Timeout = dur(-time.Second) }, "watchdog.timeout: must be positive"},
		{"zero writer wave interval", func(s *v1alpha1.FencingSLAProfileSpec) { s.Eviction.WriterWaveInterval = dur(0) }, "eviction.writerWaveInterval: must be positive"},
		{"zero count", func(s *v1alpha1.FencingSLAProfileSpec) { s.Memberlist.GossipNodes = 0 }, "memberlist.gossipNodes: must be at least 1"},
		{"controlPlaneLostAfter below the probe cycle of the proxy", func(s *v1alpha1.FencingSLAProfileSpec) { s.ControlPlaneLostAfter = dur(sec(1.6)) }, "the longest probe interval plus the probe timeout of kubernetes-api-proxy"},
		{"controlPlaneLostAfter below witnessFresherBy + maxProbeAge", func(s *v1alpha1.FencingSLAProfileSpec) { s.ControlPlaneLostAfter = dur(4100 * time.Millisecond) }, "controlPlaneLostAfter 4.1s must be at least poll.witnessFresherBy + 1.8s"},
		{"witnessFresherBy too small", func(s *v1alpha1.FencingSLAProfileSpec) { s.Poll.WitnessFresherBy = dur(sec(1.7)) }, "poll.witnessFresherBy 1.7s must be more than 1.7s"},
		{"witnessFresherBy too big", func(s *v1alpha1.FencingSLAProfileSpec) { s.Poll.WitnessFresherBy = dur(sec(2.45)) }, "poll.witnessFresherBy 2.45s must be more than 1.7s and less than controlPlaneLostAfter - 1.7s = 2.45s"},
		{"feed interval above a quarter of the watchdog timeout", func(s *v1alpha1.FencingSLAProfileSpec) { s.Watchdog.FeedInterval = dur(sec(1.26)) }, "watchdog.feedInterval 1.26s must be at most a quarter of watchdog.timeout"},
		{"watchdog timeout with a fraction of a second", func(s *v1alpha1.FencingSLAProfileSpec) { s.Watchdog.Timeout = dur(sec(2.5)) }, "watchdog.timeout must be a whole number of seconds"},
		{"suspicionMult below 3", func(s *v1alpha1.FencingSLAProfileSpec) { s.Memberlist.SuspicionMult = 2 }, "memberlist.suspicionMult must be at least 3"},
		{"probeTimeout above half of probeInterval", func(s *v1alpha1.FencingSLAProfileSpec) { s.Memberlist.ProbeTimeout = dur(101 * time.Millisecond) }, "memberlist.probeTimeout 101ms must be at most half of memberlist.probeInterval"},
		{"heartbeat longer than controlPlaneLostAfter", func(s *v1alpha1.FencingSLAProfileSpec) { s.Heartbeat.APITimeout = dur(sec(3.2)) }, "heartbeat.interval + heartbeat.apiTimeout 4.2s must be at most controlPlaneLostAfter"},
		{"heartbeat interval above a third of controlPlaneLostAfter", func(s *v1alpha1.FencingSLAProfileSpec) {
			s.Heartbeat.Interval = dur(sec(1.4))
			s.Heartbeat.APITimeout = dur(time.Second)
		}, "heartbeat.interval 1.4s must be at most a third of controlPlaneLostAfter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			tt.mutate(&spec)

			err := Validate(profileOf(spec))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate error %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// The bounds are inclusive: a profile tuned right up to them must pass.
func TestValidateAcceptsTheBounds(t *testing.T) {
	spec := validSpec()
	spec.Memberlist.ProbeTimeout = dur(100 * time.Millisecond)
	spec.Watchdog.FeedInterval = dur(750 * time.Millisecond)
	spec.ControlPlaneLostAfter = dur(4150 * time.Millisecond)
	spec.Heartbeat.Interval = dur(1380 * time.Millisecond)
	spec.Heartbeat.APITimeout = dur(2770 * time.Millisecond)

	if err := Validate(profileOf(spec)); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// The operator fixes a broken profile in one pass.
func TestValidateReportsEveryViolation(t *testing.T) {
	spec := validSpec()
	spec.Memberlist.ProbeInterval = dur(0)
	spec.Watchdog.Timeout = dur(0)

	err := Validate(profileOf(spec))
	if err == nil || !strings.Contains(err.Error(), "memberlist.probeInterval") || !strings.Contains(err.Error(), "watchdog.timeout") {
		t.Errorf("Validate error %v, want both fields", err)
	}
}

// watchdog.timeout must be a whole number of seconds: the agent does not apply 2.5 s.
func TestValidateRejectsFractionalWatchdogTimeout(t *testing.T) {
	spec := validSpec()
	spec.Watchdog.Timeout = dur(2500 * time.Millisecond)
	spec.Watchdog.FeedInterval = dur(250 * time.Millisecond)

	err := Validate(profileOf(spec))
	if err == nil || !strings.Contains(err.Error(), "whole number of seconds") {
		t.Errorf("Validate error %v, want the whole seconds rule", err)
	}
}

func TestFetchReturnsTheSpec(t *testing.T) {
	want := validSpec()
	g := &getterStub{profile: stored("sum-1", want)}

	spec, err := Fetch(t.Context(), g, "critical", "sum-1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if spec.ControlPlaneLostAfter.Duration != want.ControlPlaneLostAfter.Duration {
		t.Errorf("controlPlaneLostAfter is %s", spec.ControlPlaneLostAfter.Duration)
	}
}

// The checksum of the ConfigMap is newer than the object that GET returned.
func TestFetchRejectsAStaleAnswer(t *testing.T) {
	g := &getterStub{profile: stored("sum-1", validSpec())}

	_, err := Fetch(t.Context(), g, "critical", "sum-2")
	if err == nil || errors.Is(err, ErrInvalid) {
		t.Fatalf("Fetch error %v, want a transient error", err)
	}
}

func TestFetchRejectsAMissingChecksum(t *testing.T) {
	g := &getterStub{profile: stored("", validSpec())}

	if _, err := Fetch(t.Context(), g, "critical", ""); err == nil {
		t.Fatal("Fetch accepted a profile without a checksum to compare")
	}
}

func TestFetchMarksAnInvalidProfile(t *testing.T) {
	spec := validSpec()
	spec.Memberlist.SuspicionMult = 2
	g := &getterStub{profile: stored("sum-1", spec)}

	_, err := Fetch(t.Context(), g, "critical", "sum-1")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Fetch error %v, want ErrInvalid", err)
	}
}

func TestFetchPassesTheGetError(t *testing.T) {
	want := errors.New("connection refused")
	g := &getterStub{err: want}

	if _, err := Fetch(t.Context(), g, "critical", "sum-1"); !errors.Is(err, want) {
		t.Fatalf("Fetch error %v, want %v", err, want)
	}
}
