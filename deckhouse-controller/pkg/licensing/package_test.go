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
	"errors"
	"strings"
	"testing"
)

// P1, P2, P3, P11, P12, P13, P14, P15, P16, P17, P17a, P17b, P19, P20, P23:
// record level verdicts. A rejected record never rejects the package.
func TestParsePackageRecordVerdicts(t *testing.T) {
	pub, priv := newKey(t)
	ckt := "vaJPtef-vPnGAIWE19qlMPDYWGaER3zeeJM_3r1WO7E"

	base := ctxFor(pub)
	withThumb := base
	withThumb.ClusterKeyThumbprint = ckt

	cases := []struct {
		name         string
		record       map[string]any
		ctx          VerifyContext
		wantAccepted bool
		wantReason   string
	}{
		{
			name:         "P1 valid platform",
			record:       platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50, "nodes": 10})),
			ctx:          base,
			wantAccepted: true,
		},
		{
			name: "P2 another cluster",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["cluster_id"] = otherClusterID
				return r
			}(),
			ctx:        base,
			wantReason: ReasonClusterMismatch,
		},
		{
			name: "P3 thumbprint mismatch",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["cluster_key_thumbprint"] = "Zm9yZWlnbi10aHVtYnByaW50LXZhbHVlLXBsYWNlaG9s"
				return r
			}(),
			ctx:        withThumb,
			wantReason: ReasonClusterMismatch,
		},
		{
			name: "P3 thumbprint match",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["cluster_key_thumbprint"] = ckt
				return r
			}(),
			ctx:          withThumb,
			wantAccepted: true,
		},
		{
			name: "P12 Update record",
			record: func() map[string]any {
				r := platformOf(recordA, nil)
				r["type"] = "Update"
				return r
			}(),
			ctx:        base,
			wantReason: ReasonUnsupportedType,
		},
		{
			name: "P13 Support record",
			record: func() map[string]any {
				r := platformOf(recordA, nil)
				r["type"] = "Support"
				return r
			}(),
			ctx:        base,
			wantReason: ReasonUnsupportedType,
		},
		{
			name: "P14 Quantum record",
			record: func() map[string]any {
				r := platformOf(recordA, nil)
				r["type"] = "Quantum"
				return r
			}(),
			ctx:        base,
			wantReason: ReasonUnsupportedType,
		},
		{
			name: "P15 unknown field in platform.dkp",
			record: platformOf(recordA, map[string]any{
				"dkp": map[string]any{"edition": "Core", "resource_limits": map[string]any{"vCPU": 50}, "quantum_cores": 7},
			}),
			ctx:          base,
			wantAccepted: true,
		},
		{
			name: "P16 unknown key in platform",
			record: platformOf(recordA, map[string]any{
				"dkp":          map[string]any{"resource_limits": map[string]any{"vCPU": 50}},
				"applications": map[string]any{"whatever": true},
			}),
			ctx:          base,
			wantAccepted: true,
		},
		{
			name: "P17 expansions map",
			record: platformOf(recordA, map[string]any{
				"dkp": map[string]any{"resource_limits": map[string]any{"vCPU": 50}},
				"expansions": map[string]any{
					"AdvancedNetworking": map[string]any{},
					"AdvancedStorage":    map[string]any{"resource_limits": map[string]any{"storage_tb": 100}},
				},
			}),
			ctx:          base,
			wantAccepted: true,
		},
		{
			name: "P17a expansions as a list",
			record: platformOf(recordA, map[string]any{
				"dkp":        map[string]any{"resource_limits": map[string]any{"vCPU": 50}},
				"expansions": []any{"AdvancedNetworking"},
			}),
			ctx:        base,
			wantReason: ReasonSchemaViolation,
		},
		{
			name: "P17b extra_components map",
			record: platformOf(recordA, map[string]any{
				"dkp":              map[string]any{"resource_limits": map[string]any{"vCPU": 50}},
				"extra_components": map[string]any{"metallb": map[string]any{"feature_flags": []any{"metallb/BGPAdvanced"}}},
			}),
			ctx:          base,
			wantAccepted: true,
		},
		{
			name: "P19 perpetual record",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["expire_at"] = nil
				return r
			}(),
			ctx:          base,
			wantAccepted: true,
		},
		{
			name: "P20 expire_at before start_at",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["expire_at"] = "2025-01-01T00:00:00Z"
				return r
			}(),
			ctx:        base,
			wantReason: ReasonSchemaViolation,
		},
		{
			name: "expire_at equal to start_at",
			record: func() map[string]any {
				r := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
				r["expire_at"] = r["start_at"]
				return r
			}(),
			ctx:        base,
			wantReason: ReasonSchemaViolation,
		},
		{
			name: "id is not a UUID",
			record: func() map[string]any {
				r := platformOf("not-a-uuid", dkpLimits(map[string]any{"vCPU": 50}))
				return r
			}(),
			ctx:        base,
			wantReason: ReasonSchemaViolation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, statuses, err := ParsePackage(issue(t, priv, pkgOf(tc.record)), tc.ctx)
			if err != nil {
				t.Fatalf("package must not be rejected: %v", err)
			}
			if len(statuses) != 1 {
				t.Fatalf("got %d statuses", len(statuses))
			}
			got := statuses[0]
			if got.Accepted != tc.wantAccepted {
				t.Fatalf("Accepted = %v (reason %q, %s), want %v", got.Accepted, got.Reason, got.Message, tc.wantAccepted)
			}
			if !tc.wantAccepted && got.Reason != tc.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tc.wantReason)
			}
		})
	}

	// P23: an unknown field at the package level is ignored.
	payload := pkgOf(platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50})))
	payload["catalog_version"] = "2026.1"
	pkg, statuses, err := ParsePackage(issue(t, priv, payload), base)
	if err != nil {
		t.Fatalf("P23: %v", err)
	}
	if !statuses[0].Accepted || pkg.Ver != 1 {
		t.Fatalf("P23: %+v", statuses[0])
	}

	// P17, P17b: the raw bodies are preserved verbatim for the status.
	rec := platformOf(recordA, map[string]any{
		"dkp":              map[string]any{"resource_limits": map[string]any{"vCPU": 50}},
		"expansions":       map[string]any{"AdvancedStorage": map[string]any{"resource_limits": map[string]any{"storage_tb": 100}}},
		"extra_components": map[string]any{"metallb": map[string]any{}},
	})
	_, statuses, err = ParsePackage(issue(t, priv, pkgOf(rec)), base)
	if err != nil {
		t.Fatalf("raw bodies: %v", err)
	}
	if !strings.Contains(string(statuses[0].Platform.Expansions), "storage_tb") {
		t.Fatalf("expansions not preserved: %s", statuses[0].Platform.Expansions)
	}
	if !strings.Contains(string(statuses[0].Platform.ExtraComponents), "metallb") {
		t.Fatalf("extra_components not preserved: %s", statuses[0].Platform.ExtraComponents)
	}
}

// P11: one broken record does not take the healthy one down with it.
func TestParsePackageIsolatesBadRecords(t *testing.T) {
	pub, priv := newKey(t)

	broken := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
	broken["cluster_id"] = otherClusterID
	good := platformOf(recordB, dkpLimits(map[string]any{"vCPU": 40}))

	_, statuses, err := ParsePackage(issue(t, priv, pkgOf(broken, good)), ctxFor(pub))
	if err != nil {
		t.Fatalf("package must survive a bad record: %v", err)
	}
	if statuses[0].Accepted || statuses[0].Reason != ReasonClusterMismatch {
		t.Fatalf("first record: %+v", statuses[0])
	}
	if !statuses[1].Accepted {
		t.Fatalf("second record: %+v", statuses[1])
	}
}

// A7: a valid package next to one nobody signed. The bad key is rejected whole,
// the good one contributes: one broken key must not disarm the others.
func TestValidPackageSurvivesABadSignatureElsewhere(t *testing.T) {
	pub, priv := newKey(t)
	_, impostor := newKey(t)

	good := issue(t, priv, pkgOf(platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))))
	forged := issue(t, impostor, pkgOf(platformOf(recordB, dkpLimits(map[string]any{"vCPU": 40}))))

	if _, _, err := ParsePackage(forged, ctxFor(pub)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("forged package: err = %v, want ErrBadSignature", err)
	}

	_, statuses, err := ParsePackage(good, ctxFor(pub))
	if err != nil {
		t.Fatalf("valid package: %v", err)
	}
	res := Compute(input(ts("2026-02-01T00:00:00Z"), oneKey(statuses...)))
	if got := limitOf(t, res, MetricVCPU); got != 50 {
		t.Fatalf("vCPU = %d, want the 50 of the valid package alone", got)
	}
	if res.State != StateValid {
		t.Fatalf("state = %q, want the valid package to hold the policy", res.State)
	}
}

// A8: one package, two records, the second issued for another cluster. Only the
// first contributes, and the package itself is not rejected.
func TestOnlyOwnClusterRecordsContribute(t *testing.T) {
	pub, priv := newKey(t)

	mine := platformOf(recordA, dkpLimits(map[string]any{"vCPU": 50}))
	theirs := platformOf(recordB, dkpLimits(map[string]any{"vCPU": 40}))
	theirs["cluster_id"] = otherClusterID

	_, statuses, err := ParsePackage(issue(t, priv, pkgOf(mine, theirs)), ctxFor(pub))
	if err != nil {
		t.Fatalf("package must not be rejected: %v", err)
	}
	if !statuses[0].Accepted {
		t.Fatalf("own record: %+v", statuses[0])
	}
	if statuses[1].Accepted || statuses[1].Reason != ReasonClusterMismatch {
		t.Fatalf("foreign record: %+v", statuses[1])
	}

	res := Compute(input(ts("2026-02-01T00:00:00Z"), oneKey(statuses...)))
	if got := limitOf(t, res, MetricVCPU); got != 50 {
		t.Fatalf("vCPU = %d, want only the own record counted", got)
	}
	if res.Counts.Accepted != 1 || res.Counts.Rejected != 1 {
		t.Fatalf("counts = %+v", res.Counts)
	}
}
