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
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Specification 9.6: a record that does not even decode is one rejected record,
// not a rejected package. Every case here would fail the unmarshal of the whole
// payload if the records were decoded together.
func TestRecordShapeIsIsolated(t *testing.T) {
	pub, priv := newKey(t)

	withLimits := func(limits map[string]any) map[string]any {
		return platformOf(recordA, dkpLimits(limits))
	}

	cases := []struct {
		name string
		// bad is the first record of the package, good is always the second.
		bad     any
		wantID  string
		wantMsg string
	}{
		{
			name:    "a fractional limit is not an integer",
			bad:     withLimits(map[string]any{"vCPU": 40.5}),
			wantID:  recordA,
			wantMsg: "does not match the schema",
		},
		{
			name:    "a limit given as a string",
			bad:     withLimits(map[string]any{"vCPU": "50"}),
			wantID:  recordA,
			wantMsg: "does not match the schema",
		},
		{
			name: "grace_days given as a float",
			bad: func() map[string]any {
				r := withLimits(map[string]any{"vCPU": 50})
				r["grace_days"] = json.RawMessage("14.0")
				return r
			}(),
			wantID:  recordA,
			wantMsg: "does not match the schema",
		},
		{
			name:    "a null element",
			bad:     nil,
			wantMsg: "not a JSON object",
		},
		{
			name:    "a number where a record is expected",
			bad:     42,
			wantMsg: "not a JSON object",
		},
		{
			name:    "a string where a record is expected",
			bad:     "a license, honest",
			wantMsg: "not a JSON object",
		},
		{
			name:    "an array where a record is expected",
			bad:     []any{},
			wantMsg: "not a JSON object",
		},
		{
			name: "start_at of the wrong type",
			bad: func() map[string]any {
				r := withLimits(map[string]any{"vCPU": 50})
				r["start_at"] = 1767225600
				return r
			}(),
			wantID:  recordA,
			wantMsg: "does not match the schema",
		},
	}

	good := platformOf(recordB, dkpLimits(map[string]any{"vCPU": 40}))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, statuses, err := ParsePackage(issue(t, priv, pkgOf(tc.bad, good)), ctxFor(pub))
			if err != nil {
				t.Fatalf("package must survive an undecodable record: %v", err)
			}
			if len(statuses) != 2 {
				t.Fatalf("statuses = %d, want one per record", len(statuses))
			}
			if statuses[0].Accepted || statuses[0].Reason != ReasonSchemaViolation {
				t.Fatalf("bad record: %+v", statuses[0])
			}
			if !strings.Contains(statuses[0].Message, tc.wantMsg) {
				t.Fatalf("message = %q, want it to mention %q", statuses[0].Message, tc.wantMsg)
			}
			// Whatever a loose decode could recover must reach the status: the
			// customer has to be able to find the offending record.
			if statuses[0].ID != tc.wantID {
				t.Fatalf("recovered id = %q, want %q", statuses[0].ID, tc.wantID)
			}
			if tc.wantID != "" && statuses[0].Type != TypePlatform {
				t.Fatalf("recovered type = %q, want %q", statuses[0].Type, TypePlatform)
			}
			if !statuses[1].Accepted {
				t.Fatalf("the healthy record must still contribute: %+v", statuses[1])
			}
		})
	}
}

// licenses of the wrong shape is a package level statement, not a record level
// one: there is no record list to isolate anything within.
func TestLicensesMustBeAnArray(t *testing.T) {
	pub, priv := newKey(t)

	for _, licenses := range []any{map[string]any{"a": 1}, "none", 7} {
		payload := pkgOf()
		payload["licenses"] = licenses
		_, _, err := ParsePackage(issue(t, priv, payload), ctxFor(pub))
		if !errors.Is(err, ErrMalformed) {
			t.Fatalf("licenses = %v: err = %v, want ErrMalformed", licenses, err)
		}
	}
}

// M3, L4: values that decode fine but cannot mean anything.
func TestNonsensicalValuesAreSchemaViolations(t *testing.T) {
	pub, priv := newKey(t)

	cases := []struct {
		name    string
		record  map[string]any
		wantMsg string
	}{
		{
			name:    "a negative limit",
			record:  platformOf(recordA, dkpLimits(map[string]any{"vCPU": -1})),
			wantMsg: `resource_limits["vCPU"] is -1`,
		},
		{
			name: "a negative grace period",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["grace_days"] = -3
				return r
			}(),
			wantMsg: "grace_days is -3",
		},
		{
			name: "no start_at at all",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				delete(r, "start_at")
				return r
			}(),
			wantMsg: "start_at is missing",
		},
		{
			name: "a zero start_at",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["start_at"] = "0001-01-01T00:00:00Z"
				return r
			}(),
			wantMsg: "start_at is missing",
		},
		{
			name: "expire_at before start_at",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["expire_at"] = "2025-01-01T00:00:00Z"
				return r
			}(),
			wantMsg: "is not after start_at",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, statuses, err := ParsePackage(issue(t, priv, pkgOf(tc.record)), ctxFor(pub))
			if err != nil {
				t.Fatalf("package must not be rejected: %v", err)
			}
			if statuses[0].Accepted || statuses[0].Reason != ReasonSchemaViolation {
				t.Fatalf("status = %+v", statuses[0])
			}
			if !strings.Contains(statuses[0].Message, tc.wantMsg) {
				t.Fatalf("message = %q, want it to mention %q", statuses[0].Message, tc.wantMsg)
			}
		})
	}
}

// A zero limit is a real grant of nothing and must survive the negativity check.
func TestZeroLimitIsAccepted(t *testing.T) {
	pub, priv := newKey(t)

	_, statuses, err := ParsePackage(
		issue(t, priv, pkgOf(platformOf(recordA, dkpLimits(map[string]any{"vCPU": 0, "nodes": nil})))),
		ctxFor(pub))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !statuses[0].Accepted {
		t.Fatalf("status = %+v", statuses[0])
	}
}
