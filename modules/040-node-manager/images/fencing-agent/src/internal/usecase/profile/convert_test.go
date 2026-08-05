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

	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func validProfile() *v1alpha1.FencingSLAProfile {
	return &v1alpha1.FencingSLAProfile{
		Spec: v1alpha1.FencingSLAProfileSpec{
			ReactionGoal:          "10s",
			DetectionWindowTarget: "3s",
			Memberlist: v1alpha1.FencingSLAProfileMemberlist{
				ProbeInterval:           "300ms",
				ProbeTimeout:            "120ms",
				SuspicionMult:           3,
				SuspicionMaxTimeoutMult: 6,
				IndirectChecks:          3,
				AwarenessMaxMultiplier:  8,
				GossipInterval:          "200ms",
				RetransmitMult:          4,
				GossipToTheDeadTime:     "5s",
			},
			Fallback: v1alpha1.FencingSLAProfileFallback{
				Heartbeat:            "1s",
				TTL:                  "4s",
				KubernetesAPITimeout: "2s",
			},
			Rejoin:     v1alpha1.FencingSLAProfileRejoin{Interval: "1s", MaxInterval: "10s"},
			Evacuation: v1alpha1.FencingSLAProfileEvacuation{Delay: "6s"},
			Watchdog:   v1alpha1.FencingSLAProfileWatchdog{FeedInterval: "1s", Timeout: "10s"},
		},
	}
}

func TestConvertMapsEveryConsumedField(t *testing.T) {
	sla, err := Convert(validProfile())
	if err != nil {
		t.Fatalf("convert valid profile: %v", err)
	}

	ml := sla.Memberlist
	if ml.ProbeInterval != 300*time.Millisecond || ml.ProbeTimeout != 120*time.Millisecond ||
		ml.SuspicionMult != 3 || ml.SuspicionMaxTimeoutMult != 6 || ml.IndirectChecks != 3 ||
		ml.AwarenessMaxMultiplier != 8 || ml.GossipInterval != 200*time.Millisecond ||
		ml.RetransmitMult != 4 || ml.GossipToTheDeadTime != 5*time.Second {
		t.Errorf("memberlist tuning mismatch: %+v", ml)
	}

	if sla.Fallback.Heartbeat != time.Second || sla.Fallback.APITimeout != 2*time.Second {
		t.Errorf("fallback tuning mismatch: %+v", sla.Fallback)
	}

	if sla.Rejoin.Interval != time.Second || sla.Rejoin.MaxInterval != 10*time.Second {
		t.Errorf("rejoin tuning mismatch: %+v", sla.Rejoin)
	}

	if sla.Watchdog.FeedInterval != time.Second || sla.Watchdog.Timeout != 10*time.Second {
		t.Errorf("watchdog tuning mismatch: %+v", sla.Watchdog)
	}
}

// The CRD pattern admits zero durations and an object may predate the CEL
// rules, so the agent must reject them itself.
func TestConvertRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(p *v1alpha1.FencingSLAProfile)
		wantSub string
	}{
		{
			name:    "zero duration",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Memberlist.ProbeInterval = "0ms" },
			wantSub: "memberlist.probeInterval",
		},
		{
			name:    "garbage duration",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Watchdog.Timeout = "soon" },
			wantSub: "watchdog.timeout",
		},
		{
			name:    "zero count",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Memberlist.SuspicionMult = 0 },
			wantSub: "memberlist.suspicionMult",
		},
		{
			name:    "probe timeout not below probe interval",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Memberlist.ProbeTimeout = "300ms" },
			wantSub: "probeTimeout",
		},
		{
			name:    "heartbeat not below ttl",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.Heartbeat = "4s" },
			wantSub: "heartbeat",
		},
		{
			name:    "api timeout not below ttl",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Fallback.KubernetesAPITimeout = "4s" },
			wantSub: "kubernetesAPITimeout",
		},
		{
			name:    "rejoin interval above max",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Rejoin.Interval = "20s" },
			wantSub: "rejoin.interval",
		},
		{
			name:    "feed interval not below watchdog timeout",
			mutate:  func(p *v1alpha1.FencingSLAProfile) { p.Spec.Watchdog.FeedInterval = "10s" },
			wantSub: "feedInterval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(p)

			_, err := Convert(p)
			if err == nil {
				t.Fatal("expected an error, the agent must fail closed on an invalid profile")
			}

			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not mention %q", err, tt.wantSub)
			}
		})
	}
}

func TestConvertRejectsNilProfile(t *testing.T) {
	_, err := Convert(nil)
	if err == nil {
		t.Fatal("expected an error for a nil profile")
	}
}

// TestConvertAccumulatesAllViolations pins the accumulate-then-errors.Join
// behavior: an operator fixes a broken profile in one pass, not one field per
// CrashLoop restart.
func TestConvertAccumulatesAllViolations(t *testing.T) {
	p := validProfile()
	p.Spec.Memberlist.ProbeInterval = "0ms"
	p.Spec.Watchdog.Timeout = "soon"

	_, err := Convert(p)
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

// TestShippedPresetsConvert guards the built-in profiles in module templates
// against values the agent would reject at startup (e.g. a zero duration).
func TestShippedPresetsConvert(t *testing.T) {
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

	converted := 0
	for _, doc := range docs {
		if !strings.Contains(doc, "kind: FencingSLAProfile") {
			continue
		}

		var p v1alpha1.FencingSLAProfile
		if err := yaml.Unmarshal([]byte(doc), &p); err != nil {
			t.Fatalf("unmarshal preset: %v\n%s", err, doc)
		}

		if _, err := Convert(&p); err != nil {
			t.Errorf("shipped preset %q is rejected by the agent: %v", p.Name, err)
		}

		converted++
	}

	if converted != 4 {
		t.Errorf("expected 4 built-in presets, converted %d", converted)
	}
}
