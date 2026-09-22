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
	"os"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func dur(d time.Duration) metav1.Duration {
	return metav1.Duration{Duration: d}
}

func validProfile() *v1alpha1.FencingSLAProfile {
	return &v1alpha1.FencingSLAProfile{
		Spec: v1alpha1.FencingSLAProfileSpec{
			ReactionGoal:          "10s",
			DetectionWindowTarget: "3s",
			Memberlist: v1alpha1.FencingSLAProfileMemberlist{
				ProbeInterval:           dur(300 * time.Millisecond),
				ProbeTimeout:            dur(120 * time.Millisecond),
				SuspicionMult:           3,
				SuspicionMaxTimeoutMult: 6,
				IndirectChecks:          3,
				AwarenessMaxMultiplier:  8,
				GossipInterval:          dur(200 * time.Millisecond),
				RetransmitMult:          4,
				GossipToTheDeadTime:     dur(5 * time.Second),
			},
			Fallback: v1alpha1.FencingSLAProfileFallback{
				Heartbeat:            dur(time.Second),
				TTL:                  dur(8 * time.Second),
				KubernetesAPITimeout: dur(2 * time.Second),
			},
			Rejoin:     v1alpha1.FencingSLAProfileRejoin{Interval: dur(time.Second), MaxInterval: dur(10 * time.Second)},
			Evacuation: v1alpha1.FencingSLAProfileEvacuation{Delay: dur(6 * time.Second)},
			Watchdog:   v1alpha1.FencingSLAProfileWatchdog{FeedInterval: dur(250 * time.Millisecond), Timeout: dur(3 * time.Second)},
		},
	}
}

func TestValidateAcceptsValidProfile(t *testing.T) {
	if err := Validate(validProfile()); err != nil {
		t.Fatalf("validate a valid profile: %v", err)
	}
}

// metav1.Duration decodes the same strings the CRD pattern validates; this pins
// the contract they share.
func TestSpecDecodesDurationsFromWireStrings(t *testing.T) {
	const doc = `
spec:
  memberlist:
    probeInterval: 300ms
    gossipToTheDeadTime: 5s
  watchdog:
    timeout: 1m
`

	var p v1alpha1.FencingSLAProfile
	if err := yaml.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.Spec.Memberlist.ProbeInterval.Duration != 300*time.Millisecond {
		t.Errorf("probeInterval is %s, want 300ms", p.Spec.Memberlist.ProbeInterval.Duration)
	}

	if p.Spec.Memberlist.GossipToTheDeadTime.Duration != 5*time.Second {
		t.Errorf("gossipToTheDeadTime is %s, want 5s", p.Spec.Memberlist.GossipToTheDeadTime.Duration)
	}

	if p.Spec.Watchdog.Timeout.Duration != time.Minute {
		t.Errorf("watchdog.timeout is %s, want 1m", p.Spec.Watchdog.Timeout.Duration)
	}
}

// A malformed duration never reaches Validate: decoding rejects it first, which
// is where the agent must fail.
func TestSpecRejectsMalformedDuration(t *testing.T) {
	const doc = `
spec:
  watchdog:
    timeout: soon
`

	var p v1alpha1.FencingSLAProfile
	if err := yaml.Unmarshal([]byte(doc), &p); err == nil {
		t.Fatal("expected a decode error for a malformed duration")
	}
}

// The CRD pattern admits zero durations and an object may predate the CEL
// rules, so the agent must reject them itself.
func TestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(p *v1alpha1.FencingSLAProfile)
		wantSub string
	}{
		{
			name:    "zero duration",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Memberlist.ProbeInterval = dur(0) },
			wantSub: "memberlist.probeInterval",
		},
		{
			name:    "negative duration",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Watchdog.Timeout = dur(-time.Second) },
			wantSub: "watchdog.timeout",
		},
		{
			name:    "zero count",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Memberlist.SuspicionMult = 0 },
			wantSub: "memberlist.suspicionMult",
		},
		{
			name:    "probe timeout not below half the probe interval",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Memberlist.ProbeTimeout = dur(150 * time.Millisecond) },
			wantSub: "memberlist.probeTimeout 150ms must be less than half",
		},
		{
			name:    "sub-millisecond heartbeat",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.Heartbeat = dur(500 * time.Microsecond) },
			wantSub: "fallback.heartbeat",
		},
		{
			name:    "heartbeat not below ttl",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.Heartbeat = dur(8 * time.Second) },
			wantSub: "fallback.heartbeat 8s must be less than fallback.ttl",
		},
		{
			name:    "api timeout not below ttl",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.KubernetesAPITimeout = dur(8 * time.Second) },
			wantSub: "fallback.kubernetesAPITimeout 8s must be less than fallback.ttl",
		},
		{
			name:    "rejoin interval above max",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Rejoin.Interval = dur(20 * time.Second) },
			wantSub: "rejoin.interval",
		},
		{
			name:    "feed interval above half the watchdog timeout",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Watchdog.FeedInterval = dur(1600 * time.Millisecond) },
			wantSub: "watchdog.feedInterval 1.6s must be at most half",
		},
		{
			name:    "watchdog timeout below the floor",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Watchdog.Timeout = dur(time.Second) },
			wantSub: "watchdog.timeout: must be at least 2s",
		},
		{
			name:    "watchdog timeout with a fraction of a second",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Watchdog.Timeout = dur(2500 * time.Millisecond) },
			wantSub: "watchdog.timeout: must be a whole number of seconds",
		},
		{
			name:    "evacuation delay leaves no room for the watchdog",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Evacuation.Delay = dur(5 * time.Second) },
			wantSub: "evacuation.delay 5s must be at least",
		},
		{
			name:    "ttl leaves no room for the watchdog",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.TTL = dur(7 * time.Second) },
			wantSub: "fallback.ttl 7s must be at least",
		},
		{
			// The bound takes the longer of heartbeat and API timeout: here the
			// heartbeat, which the API-timeout-only form would miss.
			name:    "ttl leaves no room after a slow heartbeat",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.Heartbeat = dur(3 * time.Second) },
			wantSub: "fallback.ttl 8s must be at least",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(p)

			err := Validate(p)
			if err == nil {
				t.Fatal("expected an error, the agent must fail closed on an invalid profile")
			}

			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not mention %q", err, tt.wantSub)
			}
		})
	}
}

// The CRD rules allow equality at every bound; a profile tuned right up to them
// must still start.
func TestValidateAcceptsProfileAtTheBounds(t *testing.T) {
	p := validProfile()
	p.Spec.Memberlist.ProbeTimeout = dur(149 * time.Millisecond)
	p.Spec.Watchdog.FeedInterval = dur(1500 * time.Millisecond)
	p.Spec.Evacuation.Delay = dur(5500 * time.Millisecond)
	p.Spec.Fallback.TTL = dur(7500 * time.Millisecond)

	if err := Validate(p); err != nil {
		t.Fatalf("validate a profile at the bounds: %v", err)
	}
}

func TestValidateRejectsNilProfile(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("expected an error for a nil profile")
	}
}

// An operator must be able to fix a broken profile in one pass, not one field per
// CrashLoop restart.
func TestValidateAccumulatesAllViolations(t *testing.T) {
	p := validProfile()
	p.Spec.Memberlist.ProbeInterval = dur(0)
	p.Spec.Watchdog.Timeout = dur(0)

	err := Validate(p)
	if err == nil {
		t.Fatal("expected an error, the agent must fail closed on an invalid profile")
	}

	if !strings.Contains(err.Error(), "memberlist.probeInterval") {
		t.Errorf("error %q does not mention memberlist.probeInterval", err)
	}

	if !strings.Contains(err.Error(), "watchdog.timeout") {
		t.Errorf("error %q does not mention watchdog.timeout", err)
	}
}

// Guards the built-in profiles in the module templates against values the agent
// would reject at startup, such as a zero duration.
func TestShippedPresetsAreValid(t *testing.T) {
	const presets = "../../../../../../templates/fencing-agent/sla-profiles.yaml"

	raw, err := os.ReadFile(presets)
	if err != nil {
		// The file exists in a repo checkout; a build sandbox may cut the tree at src/.
		t.Skipf("shipped presets are not reachable: %v", err)
	}

	var clean []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "{{") {
			clean = append(clean, line)
		}
	}

	docs := strings.Split(strings.Join(clean, "\n"), "\n---\n")

	validated := 0
	for _, doc := range docs {
		if !strings.Contains(doc, "kind: FencingSLAProfile") {
			continue
		}

		var p v1alpha1.FencingSLAProfile
		if err := yaml.Unmarshal([]byte(doc), &p); err != nil {
			t.Fatalf("unmarshal preset: %v\n%s", err, doc)
		}

		if err := Validate(&p); err != nil {
			t.Errorf("shipped preset %q is rejected by the agent: %v", p.Name, err)
		}

		validated++
	}

	if validated != 4 {
		t.Errorf("expected 4 built-in presets, validated %d", validated)
	}
}
