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
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// P4, P5, P6, P7, P8, P10, P21, P22: package level rejections.
func TestParsePackageRejections(t *testing.T) {
	pub, priv := newKey(t)
	_, otherPriv := newKey(t)
	ctx := ctxFor(pub)
	record := platformOf(recordA, platformLimits(map[string]any{"vCPU": 50}))

	unsigned := func(header map[string]any, payload any) string {
		hb, _ := marshalCompact(header)
		pb, _ := marshalCompact(payload)
		return base64.RawURLEncoding.EncodeToString(hb) + "." +
			base64.RawURLEncoding.EncodeToString(pb) + "." +
			base64.RawURLEncoding.EncodeToString([]byte("not-a-signature"))
	}

	cases := []struct {
		name    string
		token   string
		wantErr error
		wantMsg string
	}{
		{
			name: "P4 wrong issuer",
			token: issue(t, priv, func() map[string]any {
				p := pkgOf(record)
				p["iss"] = "license.evil.example"
				return p
			}()),
			wantErr: ErrMalformed,
			wantMsg: "iss",
		},
		{
			name: "P5 registration typ",
			token: func() string {
				s, err := Sign(map[string]any{"typ": TypRegistration}, pkgOf(record), priv)
				if err != nil {
					t.Fatal(err)
				}
				return s
			}(),
			wantErr: ErrWrongType,
			wantMsg: `typ is "deckhouse-cluster-license+jwt", want "deckhouse-license-key+jwt"`,
		},
		{
			name:    "P6 alg none",
			token:   unsigned(map[string]any{"alg": "none", "typ": TypLicense}, pkgOf(record)),
			wantErr: ErrUnsupportedAlg,
		},
		{
			name:    "P6 alg HS256",
			token:   unsigned(map[string]any{"alg": "HS256", "typ": TypLicense}, pkgOf(record)),
			wantErr: ErrUnsupportedAlg,
		},
		{
			name:    "P7 payload tampered",
			token:   tamper(t, issue(t, priv, pkgOf(record))),
			wantErr: ErrBadSignature,
		},
		{
			name:    "signed by an unknown key",
			token:   issue(t, otherPriv, pkgOf(record)),
			wantErr: ErrBadSignature,
		},
		{
			name:    "P8 packed blob",
			token:   "not-a-jwt-opaque-packed-blob-EXAMPLE",
			wantErr: ErrMalformed,
		},
		{
			name:    "P10 empty licenses",
			token:   issue(t, priv, pkgOf()),
			wantErr: ErrMalformed,
			wantMsg: "licenses is empty",
		},
		{
			name: "P21 newer schema version",
			token: issue(t, priv, func() map[string]any {
				p := pkgOf(record)
				p["ver"] = 2
				return p
			}()),
			wantErr: ErrMalformed,
			wantMsg: "package schema version 2 is newer than supported by this Deckhouse release (max 1); update Deckhouse",
		},
		{
			name: "P22 missing ver",
			token: issue(t, priv, func() map[string]any {
				p := pkgOf(record)
				delete(p, "ver")
				return p
			}()),
			wantErr: ErrMalformed,
			wantMsg: "missing ver",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParsePackage(tc.token, ctx)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.wantMsg)
			}
		})
	}

	// An empty vendor key list rejects everything.
	_, _, err := ParsePackage(issue(t, priv, pkgOf(record)), VerifyContext{ClusterID: testClusterID})
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("no vendor keys: error = %v, want %v", err, ErrBadSignature)
	}
}

// tamper flips one byte of the payload while keeping the token well formed.
func tamper(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(raw), `"vCPU":50`, `"vCPU":99`, 1)
	if patched == string(raw) {
		t.Fatal("payload was not tampered with")
	}
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(patched))
	return strings.Join(parts, ".")
}

// P9: the token survives line wraps, NBSP and zero width characters.
func TestParsePackageNormalizesInput(t *testing.T) {
	pub, priv := newKey(t)
	token := issue(t, priv, pkgOf(platformOf(recordA, platformLimits(map[string]any{"vCPU": 50}))))

	mangled := token[:20] + "\n  " + token[20:40] + "\u00a0" + token[40:60] + "\u200b\ufeff" + token[60:]
	_, statuses, err := ParsePackage(mangled, ctxFor(pub))
	if err != nil {
		t.Fatalf("mangled token rejected: %v", err)
	}
	if !statuses[0].Accepted {
		t.Fatalf("record rejected: %+v", statuses[0])
	}
}

// P14a and the forward compatibility guarantee: a package mixing an unknown
// record type with a Platform is accepted and the Platform still contributes.
func TestUnknownTypeDoesNotBlockPlatform(t *testing.T) {
	pub, priv := newKey(t)

	future := platformOf(recordC, nil)
	future["type"] = "Quantum"
	future["quantum_entanglement"] = map[string]any{"qubits": 42}
	work := platformOf(recordA, platformLimits(map[string]any{"vCPU": 50, "nodes": 10}))

	_, statuses, err := ParsePackage(issue(t, priv, pkgOf(future, work)), ctxFor(pub))
	if err != nil {
		t.Fatalf("package rejected: %v", err)
	}
	if statuses[0].Accepted || statuses[0].Reason != ReasonUnsupportedType {
		t.Fatalf("unknown type: %+v", statuses[0])
	}
	if !statuses[1].Accepted {
		t.Fatalf("platform: %+v", statuses[1])
	}

	res := Compute(input(ts("2026-02-01T00:00:00Z"), oneKey(statuses...)))
	if got := limitOf(t, res, MetricVCPU); got != 50 {
		t.Fatalf("vCPU = %d, want 50", got)
	}
	if res.State != StateValid {
		t.Fatalf("state = %q, want %q", res.State, StateValid)
	}

	// P14a: a package where every record is of an unknown type is still accepted.
	_, statuses, err = ParsePackage(issue(t, priv, pkgOf(future)), ctxFor(pub))
	if err != nil {
		t.Fatalf("all-unknown package rejected: %v", err)
	}
	if statuses[0].Accepted {
		t.Fatalf("unknown type accepted: %+v", statuses[0])
	}
}

// P18: a revoked record is rejected, and a commercial reason means Violation
// without any grace.
func TestRevokedRecords(t *testing.T) {
	pub, priv := newKey(t)
	record := platformOf(recordA, platformLimits(map[string]any{"vCPU": 50}))
	token := issue(t, priv, pkgOf(record))
	now := ts("2026-02-01T00:00:00Z")

	cases := []struct {
		name      string
		reason    string
		wantState string
	}{
		{"contract terminated", ReasonContractTerminated, StateViolation},
		{"non payment", ReasonNonPayment, StateViolation},
		{"abuse", ReasonAbuse, StateViolation},
		// A reissue is a technical event: the record stops contributing, but the
		// state is computed from whatever else is installed.
		{"reissued", ReasonReissued, StateViolation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ctxFor(pub)
			ctx.Revoked = map[string]Revocation{recordA: {RevokedAt: now, Reason: tc.reason}}

			_, statuses, err := ParsePackage(token, ctx)
			if err != nil {
				t.Fatalf("package rejected: %v", err)
			}
			if statuses[0].Accepted || statuses[0].Reason != ReasonRevoked {
				t.Fatalf("status = %+v", statuses[0])
			}
			if statuses[0].RevokedReason != tc.reason {
				t.Fatalf("RevokedReason = %q, want %q", statuses[0].RevokedReason, tc.reason)
			}
			if got := Compute(input(now, oneKey(statuses...))).State; got != tc.wantState {
				t.Fatalf("state = %q, want %q", got, tc.wantState)
			}
		})
	}

	// Reissued leaves the state to the surviving records; a commercial reason
	// does not, even with a healthy record next to it.
	healthy := wl(recordB, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(40)})
	ctx := ctxFor(pub)
	ctx.Revoked = map[string]Revocation{recordA: {RevokedAt: now, Reason: ReasonReissued}}
	_, statuses, err := ParsePackage(token, ctx)
	if err != nil {
		t.Fatalf("package rejected: %v", err)
	}
	res := Compute(input(now, oneKey(append(statuses, healthy)...)))
	if res.State != StateValid {
		t.Fatalf("reissued state = %q, want %q", res.State, StateValid)
	}
	if got := limitOf(t, res, MetricVCPU); got != 40 {
		t.Fatalf("vCPU = %d, want 40", got)
	}

	ctx.Revoked = map[string]Revocation{recordA: {RevokedAt: now, Reason: ReasonNonPayment}}
	_, statuses, _ = ParsePackage(token, ctx)
	res = Compute(input(now, oneKey(append(statuses, healthy)...)))
	if res.State != StateViolation {
		t.Fatalf("non payment state = %q, want %q", res.State, StateViolation)
	}
}

func TestRecordIDsUnverified(t *testing.T) {
	_, priv := newKey(t)
	token := issue(t, priv, pkgOf(
		platformOf(recordA, platformLimits(map[string]any{"vCPU": 50})),
		platformOf(recordB, platformLimits(map[string]any{"vCPU": 40})),
	))

	ids, err := RecordIDsUnverified(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ids) != 2 || ids[0] != recordA || ids[1] != recordB {
		t.Fatalf("ids = %v", ids)
	}
	if _, err := RecordIDsUnverified("garbage"); !errors.Is(err, ErrMalformed) {
		t.Fatalf("error = %v, want %v", err, ErrMalformed)
	}
}

func TestParsePublicKey(t *testing.T) {
	pub, _ := newKey(t)
	encoded := base64.RawURLEncoding.EncodeToString(pub)

	got, err := ParsePublicKey(encoded)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.Equal(pub) {
		t.Fatal("round trip mismatch")
	}
	if _, err := ParsePublicKey("!!!"); err == nil {
		t.Fatal("want an error for a non base64url key")
	}
	if _, err := ParsePublicKey(base64.RawURLEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("want an error for a key of the wrong length")
	}
}

// The embedded vendor list carries exactly the MVP test key, every entry parses,
// and a package signed by anyone else is rejected against the default list.
func TestEmbeddedVendorLists(t *testing.T) {
	if len(VendorPublicKeys) != len(vendorPublicKeysB64) || len(VendorPublicKeys) == 0 {
		t.Fatalf("VendorPublicKeys = %d entries, want %d", len(VendorPublicKeys), len(vendorPublicKeysB64))
	}
	for i, key := range VendorPublicKeys {
		if len(key) != ed25519.PublicKeySize {
			t.Fatalf("VendorPublicKeys[%d] is %d bytes", i, len(key))
		}
	}
	if len(RevokedRecords) != 0 {
		t.Fatalf("RevokedRecords = %d entries, the MVP ships an empty list", len(RevokedRecords))
	}

	_, stranger := newKey(t)
	token := issue(t, stranger, pkgOf(platformOf(recordA, platformLimits(map[string]any{"vCPU": 50}))))
	_, _, err := ParsePackage(token, VerifyContext{VendorKeys: VendorPublicKeys, ClusterID: testClusterID})
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("stranger-signed package: got %v, want ErrBadSignature", err)
	}
}
