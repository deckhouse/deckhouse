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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// D7: the thumbprint matches an independently computed RFC 7638 reference and
// stays byte stable for a fixed key.
func TestThumbprintMatchesRFC7638(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)

	// Reference: SHA-256 over the canonical JWK with the three required members
	// in lexicographic order, base64url without padding.
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` + base64.RawURLEncoding.EncodeToString(pub) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	want := base64.RawURLEncoding.EncodeToString(sum[:])

	if got := Thumbprint(pub); got != want {
		t.Fatalf("Thumbprint() = %q, want %q", got, want)
	}
	// Golden value for the seed 00..1f, so the canonical form cannot drift even
	// if the reference computation above drifts with it.
	const golden = "1IG2tMH7J2wbJZnOf8LJzQitKf7LMvoAElsuDMVM54Y"
	if got := Thumbprint(pub); got != golden {
		t.Fatalf("Thumbprint() = %q, want golden %q", got, golden)
	}
	// D9: base64url without padding.
	if strings.ContainsAny(golden, "=+/") {
		t.Fatalf("golden %q is not base64url without padding", golden)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "abc", "abc"},
		{"spaces and newlines", " a\n b\tc\r\n", "abc"},
		{"nbsp", "a\u00a0b", "ab"},
		{"zero width", "a\u200bb\ufeffc", "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// D1, D2: the header carries jwk while nothing is accepted yet and kid
// afterwards, exactly one of the two.
func TestRegistrationHeaderKeyForm(t *testing.T) {
	pub, priv := newKey(t)

	cases := []struct {
		name       string
		includeJWK bool
		records    []string
		wantJWK    bool
	}{
		{"D1 no accepted packages", true, nil, true},
		{"D2 accepted packages", false, []string{recordB, recordA}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := BuildRegistrationRequest(RegistrationInput{
				ClusterID:  testClusterID,
				JTI:        recordC,
				IssuedAt:   ts("2026-09-09T12:00:00Z"),
				Seq:        3,
				Records:    tc.records,
				Key:        priv,
				IncludeJWK: tc.includeJWK,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			parsed, err := Parse(token)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := parsed.Header["typ"]; got != TypRegistration {
				t.Fatalf("typ = %v", got)
			}
			_, hasJWK := parsed.Header["jwk"]
			_, hasKID := parsed.Header["kid"]
			if tc.wantJWK && (!hasJWK || hasKID) {
				t.Fatalf("header = %v, want jwk only", parsed.Header)
			}
			if !tc.wantJWK {
				if hasJWK || !hasKID {
					t.Fatalf("header = %v, want kid only", parsed.Header)
				}
				if parsed.Header["kid"] != Thumbprint(pub) {
					t.Fatalf("kid = %v, want %q", parsed.Header["kid"], Thumbprint(pub))
				}
			}
			if !parsed.Verify(pub) {
				t.Fatal("signature does not verify")
			}
		})
	}
	// The jwk member itself is the three member canonical form.
	token, err := BuildRegistrationRequest(RegistrationInput{ClusterID: testClusterID, IssuedAt: time.Now(), Key: priv, IncludeJWK: true})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	parsed, err := Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	jwk, ok := parsed.Header["jwk"].(map[string]any)
	if !ok {
		t.Fatalf("jwk = %T", parsed.Header["jwk"])
	}
	want := map[string]any{"crv": "Ed25519", "kty": "OKP", "x": base64.RawURLEncoding.EncodeToString(pub)}
	for k, v := range want {
		if jwk[k] != v {
			t.Fatalf("jwk[%q] = %v, want %v", k, jwk[k], v)
		}
	}
	if len(jwk) != len(want) {
		t.Fatalf("jwk = %v, want exactly %v", jwk, want)
	}
}

// D5, D8, D9, D10: serialization rules of the registration payload.
func TestRegistrationPayloadSerialization(t *testing.T) {
	_, priv := newKey(t)

	token, err := BuildRegistrationRequest(RegistrationInput{
		ClusterID:    testClusterID,
		PublicDomain: "a<b>c&d.example.com",
		JTI:          recordC,
		IssuedAt:     ts("2026-09-09T12:00:00Z").In(time.FixedZone("MSK", 3*3600)),
		Seq:          1,
		Metrics:      map[string]int64{MetricServers: 24, MetricVCPU: 384, MetricCores: 192},
		Key:          priv,
		IncludeJWK:   true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// D9: base64url, no padding anywhere.
	if strings.Contains(token, "=") {
		t.Fatalf("token contains padding: %q", token)
	}

	parsed, err := Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw := string(parsed.Payload)

	// D8: <, > and & survive as is, the license server does not escape them.
	if !strings.Contains(raw, "a<b>c&d.example.com") {
		t.Fatalf("payload escapes HTML characters: %s", raw)
	}

	var payload map[string]any
	if err := json.Unmarshal(parsed.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}

	// D10: iat is an RFC 3339 string in UTC, not a number.
	iat, ok := payload["iat"].(string)
	if !ok {
		t.Fatalf("iat = %T, want string", payload["iat"])
	}
	if iat != "2026-09-09T12:00:00Z" {
		t.Fatalf("iat = %q, want UTC RFC 3339", iat)
	}

	// D5: the three metrics are plain integers, present even when zero, with no
	// nested objects anywhere.
	metrics, ok := payload["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("metrics = %T", payload["metrics"])
	}
	want := map[string]float64{MetricServers: 24, MetricVCPU: 384, MetricCores: 192}
	if len(metrics) != len(want) {
		t.Fatalf("metrics = %v, want exactly %v", metrics, want)
	}
	for name, value := range want {
		got, ok := metrics[name].(float64)
		if !ok {
			t.Fatalf("metrics.%s = %T, want a number", name, metrics[name])
		}
		if got != value {
			t.Fatalf("metrics.%s = %v, want %v", name, got, value)
		}
	}

	// Optional fields disappear when empty, records and ver are always there.
	for _, absent := range []string{"build", "dkp_version"} {
		if _, ok := payload[absent]; ok {
			t.Fatalf("%s must be omitted when empty", absent)
		}
	}
	if payload["ver"].(float64) != SchemaVersion {
		t.Fatalf("ver = %v", payload["ver"])
	}
	for _, always := range []string{"records", "active_keys"} {
		if _, ok := payload[always].([]any); !ok {
			t.Fatalf("%s = %T, want an array", always, payload[always])
		}
	}
}

// Record ids are sorted and the caller slice is never reordered in place.
func TestRegistrationRecordsSorted(t *testing.T) {
	_, priv := newKey(t)
	input := []string{recordC, recordA, recordB}
	original := append([]string(nil), input...)

	token, err := BuildRegistrationRequest(RegistrationInput{
		ClusterID: testClusterID,
		IssuedAt:  ts("2026-09-09T12:00:00Z"),
		Records:   input,
		Key:       priv,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	parsed, _ := Parse(token)
	var payload struct {
		Records []string `json:"records"`
	}
	if err := json.Unmarshal(parsed.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	want := []string{recordB, recordC, recordA}
	for i := range want {
		if payload.Records[i] != want[i] {
			t.Fatalf("records = %v, want %v", payload.Records, want)
		}
	}
	for i := range original {
		if input[i] != original[i] {
			t.Fatalf("BuildRegistrationRequest mutated the caller slice: %v", input)
		}
	}
}

func TestSignDoesNotMutateHeader(t *testing.T) {
	_, priv := newKey(t)
	header := map[string]any{"typ": TypLicense}
	if _, err := Sign(header, map[string]any{"a": 1}, priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(header) != 1 {
		t.Fatalf("Sign mutated the caller header: %v", header)
	}
}

func TestParseRejects(t *testing.T) {
	_, priv := newKey(t)
	good := issue(t, priv, pkgOf())

	cases := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrMalformed},
		{"two segments", "aa.bb", ErrMalformed},
		{"empty signature", strings.Join(strings.Split(good, ".")[:2], ".") + ".", ErrMalformed},
		{"not base64", "!!!.bb.cc", ErrMalformed},
		{"header not json", base64.RawURLEncoding.EncodeToString([]byte("nope")) + ".bb.cc", ErrMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("Parse() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// A key of the wrong size is refused instead of panicking inside crypto.
func TestInvalidKeys(t *testing.T) {
	if _, err := Sign(nil, map[string]any{}, ed25519.PrivateKey("short")); err == nil {
		t.Fatal("Sign accepted a truncated private key")
	}
	if _, err := BuildRegistrationRequest(RegistrationInput{Key: ed25519.PrivateKey("short")}); err == nil {
		t.Fatal("BuildRegistrationRequest accepted a truncated private key")
	}
	_, priv := newKey(t)
	token, err := Sign(map[string]any{"typ": TypLicense}, map[string]any{}, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parsed, err := Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Verify(ed25519.PublicKey("short")) {
		t.Fatal("Verify accepted a truncated public key")
	}
}

// D4: two consecutive requests. The jti is fresh every time so the license
// server can tell a retry from a new request, and seq never goes backwards.
func TestConsecutiveRequestsDifferInJTIAndSeq(t *testing.T) {
	_, priv := newKey(t)

	payloadOf := func(token string) map[string]any {
		t.Helper()
		parsed, err := Parse(token)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		var p map[string]any
		if err := json.Unmarshal(parsed.Payload, &p); err != nil {
			t.Fatalf("payload: %v", err)
		}
		return p
	}

	in := RegistrationInput{
		ClusterID: testClusterID,
		JTI:       "8f14e45f-ceea-467a-9575-1c1d0a0b1c2d",
		Seq:       7,
		IssuedAt:  ts("2026-05-01T00:00:00Z"),
		Key:       priv,
	}
	first, err := BuildRegistrationRequest(in)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}

	in.JTI = "2c1743a3-91b8-4c4a-b0dd-5f6a7b8c9d01"
	in.Seq = 8
	in.IssuedAt = in.IssuedAt.Add(time.Hour)
	second, err := BuildRegistrationRequest(in)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}

	a, b := payloadOf(first), payloadOf(second)
	if a["jti"] == b["jti"] {
		t.Fatalf("jti = %v twice, want a fresh one per request", a["jti"])
	}
	if a["seq"].(float64) > b["seq"].(float64) {
		t.Fatalf("seq went backwards: %v then %v", a["seq"], b["seq"])
	}

	// The same input twice is byte identical: only jti, seq, iat and the
	// metrics are allowed to move the bytes.
	repeat, err := BuildRegistrationRequest(in)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if repeat != second {
		t.Fatal("the same input produced different bytes")
	}
}
