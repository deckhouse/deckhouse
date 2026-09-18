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

package licensing

import (
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

func TestSampleDue(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		last time.Duration // age of the last sample, negative for the future
		want bool
	}{
		{name: "an empty journal always samples", want: true},
		// Written by a clock that was wrong. Waiting it out would stop sampling
		// until that date actually arrives.
		{name: "a sample dated in the future is due at once", last: -3 * time.Hour, want: true},
		{name: "a sample taken ten minutes ago is too fresh", last: 10 * time.Minute},
		{name: "a sample taken 54 minutes ago is still too fresh", last: 54 * time.Minute},
		{name: "a sample taken 55 minutes ago is due", last: 55 * time.Minute, want: true},
		{name: "a sample taken two hours ago is due", last: 2 * time.Hour, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journal := new(licensing.Journal)
			if tc.last != 0 {
				journal.Add(licensing.Sample{At: now.Add(-tc.last), Values: map[string]float64{metricNodes: 1}}, retention)
			}
			if got := sampleDue(journal, now); got != tc.want {
				t.Fatalf("sampleDue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{in: 61 * 24 * time.Hour, want: "61d"},
		{in: 25 * time.Hour, want: "1d"},
		{in: 5 * time.Hour, want: "5h"},
		{in: 90 * time.Second, want: "1m"},
		{in: -time.Hour, want: "0m"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := humanDuration(tc.in); got != tc.want {
				t.Fatalf("humanDuration(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
