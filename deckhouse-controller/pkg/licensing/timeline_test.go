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

package licensing

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A2: the stacking scenario of the specification, segment by segment.
func TestTimelineStackingScenario(t *testing.T) {
	res := Compute(oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(50), "nodes": i64(10)}),
		wl(recordB, "2026-05-01T00:00:00Z", "2026-11-01T00:00:00Z", map[string]*int64{"vCPU": i64(40), "nodes": i64(5)}),
	), nil, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	if res.State != StateValid {
		t.Fatalf("state = %q", res.State)
	}
	want := []string{
		"2026-05-01T00:00:00Z..2026-07-01T00:00:00Z Valid nodes=15 vCPU=90",
		"2026-07-01T00:00:00Z..2026-11-01T00:00:00Z Valid nodes=5 vCPU=40",
		// M7: once every record has expired the metrics stay named with a
		// finite zero. An empty map would read as "nothing was ever granted".
		"2026-11-01T00:00:00Z..2026-11-15T00:00:00Z Grace nodes=0 vCPU=0",
		"2026-11-15T00:00:00Z..nil Violation nodes=0 vCPU=0",
	}
	assertTimeline(t, res.Timeline, want)

	if res.NextReduction == nil {
		t.Fatal("nextReduction is nil, want the 2026-07-01 drop")
	}
	if !res.NextReduction.At.Equal(ts("2026-07-01T00:00:00Z")) {
		t.Fatalf("nextReduction.At = %s", res.NextReduction.At)
	}
	if *res.NextReduction.From["vCPU"] != 90 || *res.NextReduction.To["vCPU"] != 40 {
		t.Fatalf("nextReduction = %+v", res.NextReduction)
	}
}

func assertTimeline(t *testing.T, got []Segment, want []string) {
	t.Helper()
	lines := make([]string, 0, len(got))
	for _, s := range got {
		to := "nil"
		if s.To != nil {
			to = s.To.UTC().Format(time.RFC3339)
		}
		line := fmt.Sprintf("%s..%s %s", s.From.UTC().Format(time.RFC3339), to, s.State)
		names := make([]string, 0, len(s.Limits))
		for m := range s.Limits {
			names = append(names, m)
		}
		sortStrings(names)
		for _, m := range names {
			if s.Limits[m] == nil {
				line += " " + m + "=unlimited"
				continue
			}
			line += fmt.Sprintf(" %s=%d", m, *s.Limits[m])
		}
		lines = append(lines, line)
	}
	if len(lines) != len(want) {
		t.Fatalf("timeline =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("segment %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// A19: two records sharing a date produce one breakpoint, not two.
func TestTimelineDeduplicatesBreakpoints(t *testing.T) {
	res := Compute(oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)}),
		wl(recordB, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(40)}),
	), nil, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	assertTimeline(t, res.Timeline, []string{
		"2026-01-01T00:00:00Z..2026-07-01T00:00:00Z Valid vCPU=90",
		"2026-07-01T00:00:00Z..2026-07-15T00:00:00Z Grace vCPU=0",
		"2026-07-15T00:00:00Z..nil Violation vCPU=0",
	})
}

// A18: a growing quota has no next reduction.
func TestNextReductionNilWhenQuotaGrows(t *testing.T) {
	res := Compute(oneKey(
		wl(recordA, "2026-01-01T00:00:00Z", "", map[string]*int64{"vCPU": i64(50)}),
		wl(recordB, "2026-09-01T00:00:00Z", "", map[string]*int64{"vCPU": i64(40)}),
	), nil, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	if res.NextReduction != nil {
		t.Fatalf("nextReduction = %+v, want nil", res.NextReduction)
	}
	assertTimeline(t, res.Timeline, []string{
		"2026-01-01T00:00:00Z..2026-09-01T00:00:00Z Valid vCPU=50",
		"2026-09-01T00:00:00Z..nil Valid vCPU=90",
	})
}
