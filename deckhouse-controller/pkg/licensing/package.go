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
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// TypePlatform is the only record type this iteration understands. Every other
// type is ignored uniformly, never rejected: packages carrying record types
// invented later must keep applying on older builds.
const TypePlatform = "Platform"

// Package is a license package as issued by the license server. Unknown fields
// are ignored by design (default encoding/json behaviour).
type Package struct {
	Ver          int    `json:"ver"`
	Iss          string `json:"iss"`
	Sub          string `json:"sub"`
	JTI          string `json:"jti"`
	CustomerName string `json:"customer_name"`
	IAT          string `json:"iat"`
	// Licenses stays raw so that one record of the wrong shape cannot fail the
	// unmarshal of the whole package (spec 9.6). Records are decoded one by one
	// by ParsePackage, which reports the outcome per record in []RecordStatus.
	Licenses []json.RawMessage `json:"licenses"`
}

// Record is a single license record. Pointer types are mandatory: "the field is
// absent" and "the field is zero" are different statements.
type Record struct {
	Type                 string     `json:"type"`
	ID                   string     `json:"id"`
	StartAt              time.Time  `json:"start_at"`
	ExpireAt             *time.Time `json:"expire_at"`
	GraceDays            *int       `json:"grace_days"`
	ClusterID            string     `json:"cluster_id"`
	PublicDomain         string     `json:"public_domain"`
	ClusterKeyThumbprint *string    `json:"cluster_key_thumbprint"`
	Origin               string     `json:"origin"`
	Platform             *Platform  `json:"platform"`
	Renews               []string   `json:"renews"`
	Supersedes           []string   `json:"supersedes"`
}

// Platform is the body of a Platform record. A nil ResourceLimits map means the
// record says nothing about quotas; an empty map means "no limits" explicitly.
type Platform struct {
	Edition        string            `json:"edition"`
	ResourceLimits map[string]*int64 `json:"resource_limits"`
	// Expansions and ExtraComponents are maps of scopes. This iteration does not
	// interpret them; it only requires them to be JSON objects, so that a
	// package issued for a newer fleet still applies here.
	Expansions      json.RawMessage `json:"expansions"`
	ExtraComponents json.RawMessage `json:"extra_components"`
}

// Revocation reasons. Only the commercial ones turn the cluster red: a reissue
// is a technical event, the customer did nothing wrong.
const (
	ReasonContractTerminated = "ContractTerminated"
	ReasonNonPayment         = "NonPayment"
	ReasonAbuse              = "Abuse"
	ReasonReissued           = "Reissued"
)

// Revocation is an entry of the embedded revocation list.
type Revocation struct {
	RevokedAt time.Time
	Reason    string
}

// RevokedRecords is the embedded revocation list, keyed by record id.
//
// ponytail: populated at build time from the license server revocation feed;
// the online channel is out of scope for this iteration, so an updated list
// only reaches air-gapped clusters with a new build.
var RevokedRecords = map[string]Revocation{}

// vendorPublicKeysB64 lists the vendor signing keys, base64url without padding.
//
// ponytail: the single entry is a TEST key generated for the MVP, because the
// license server (deckhouse-lk) does not publish a stable key yet: it derives
// its signing seed from secret_key_base at runtime and GET /v1/public-keys is
// not implemented. The matching seed is handed to the license server team so
// both sides sign and verify with the same pair during development. Production
// builds must replace this entry (or append to it) with the key published by
// the license server; this slice is the only place to change.
var vendorPublicKeysB64 = []string{
	"_urEzDooQTuRV0p7ZHB4PsUJy4XxWsbmCCMZmrokZ5U", // test key, MVP only
}

// VendorPublicKeys are the vendor signing keys embedded at build time. A package
// is valid when any of them verifies its signature.
var VendorPublicKeys = mustVendorKeys(vendorPublicKeysB64)

func mustVendorKeys(b64 []string) []ed25519.PublicKey {
	keys := make([]ed25519.PublicKey, 0, len(b64))
	for _, s := range b64 {
		key, err := ParsePublicKey(s)
		if err != nil {
			panic(fmt.Sprintf("licensing: embedded vendor key %q: %v", s, err))
		}
		keys = append(keys, key)
	}
	return keys
}

// ParsePublicKey decodes a base64url (unpadded) Ed25519 public key.
func ParsePublicKey(b64url string) (ed25519.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(b64url)
	if err != nil {
		return nil, fmt.Errorf("licensing: public key is not base64url: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("licensing: public key is %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// VerifyContext is everything package verification needs from the cluster.
type VerifyContext struct {
	VendorKeys           []ed25519.PublicKey
	ClusterID            string
	ClusterKeyThumbprint string
	Revoked              map[string]Revocation
	// Now is the verification timestamp. Record validity in time is policy, not
	// verification, so it is only used for diagnostics.
	Now time.Time
}

// Reasons why a record does not contribute to the policy. Time based reasons
// (Expired, NotYetValid) and policy level ones (Duplicate, Renewed,
// Superseded) are assigned by Compute, the rest by ParsePackage.
const (
	ReasonRenewed         = "Renewed"
	ReasonSuperseded      = "Superseded"
	ReasonExpired         = "Expired"
	ReasonDuplicate       = "Duplicate"
	ReasonNotYetValid     = "NotYetValid"
	ReasonRevoked         = "Revoked"
	ReasonClusterMismatch = "ClusterMismatch"
	ReasonSchemaViolation = "SchemaViolation"
	ReasonUnsupportedType = "UnsupportedType"
)

// RecordStatus is a record plus the verdict on it.
type RecordStatus struct {
	Record

	Accepted bool
	Reason   string
	Message  string
	// RenewedBy holds the id of the successor that extinguished this record,
	// for both Renewed and Superseded.
	RenewedBy string
	// RevokedReason is the revocation list reason when Reason is Revoked.
	RevokedReason string
}

// ParsePackage verifies a license package and checks every record it carries.
//
// A failure of the package level checks (format, signature, issuer, schema
// version, non-empty record list) rejects the whole package and returns an
// error. A failure of a record level check only rejects that record: one bad
// record must never zero out the policy.
func ParsePackage(token string, ctx VerifyContext) (*Package, []RecordStatus, error) {
	t, err := Parse(token)
	if err != nil {
		return nil, nil, err
	}
	typ, _ := t.Header["typ"].(string)
	if typ != TypLicense {
		return nil, nil, fmt.Errorf("%w: typ is %q, want %q", ErrWrongType, typ, TypLicense)
	}
	if !verifyAny(t, ctx.VendorKeys) {
		return nil, nil, fmt.Errorf("%w: no vendor key verifies this package", ErrBadSignature)
	}

	// ver is checked before the full unmarshal: a newer schema may well fail to
	// unmarshal into this build's types, and the version message is the useful one.
	var probe struct {
		Ver *int `json:"ver"`
	}
	if err := json.Unmarshal(t.Payload, &probe); err != nil {
		return nil, nil, fmt.Errorf("%w: payload is not JSON: %s", ErrMalformed, err)
	}
	if probe.Ver == nil {
		return nil, nil, fmt.Errorf("%w: missing ver", ErrMalformed)
	}
	if *probe.Ver > SchemaVersion {
		return nil, nil, fmt.Errorf("%w: package schema version %d is newer than supported by this Deckhouse release (max %d); update Deckhouse",
			ErrUnsupportedVersion, *probe.Ver, SchemaVersion)
	}

	var pkg Package
	if err := json.Unmarshal(t.Payload, &pkg); err != nil {
		return nil, nil, fmt.Errorf("%w: payload does not match schema v%d: %s", ErrMalformed, *probe.Ver, err)
	}
	if pkg.Iss != Issuer {
		return nil, nil, fmt.Errorf("%w: iss is %q, want %q", ErrMalformed, pkg.Iss, Issuer)
	}
	if len(pkg.Licenses) == 0 {
		return nil, nil, fmt.Errorf("%w: licenses is empty", ErrMalformed)
	}

	statuses := make([]RecordStatus, 0, len(pkg.Licenses))
	for _, raw := range pkg.Licenses {
		statuses = append(statuses, checkRecord(raw, ctx))
	}

	return &pkg, statuses, nil
}

func verifyAny(t *Token, keys []ed25519.PublicKey) bool {
	for _, k := range keys {
		if t.Verify(k) {
			return true
		}
	}
	return false
}

// checkRecord decodes one record and applies the record level checks of spec
// 9.3. Decoding is per record on purpose: a record of the wrong shape becomes a
// SchemaViolation status, it never fails the package (spec 9.6).
func checkRecord(raw json.RawMessage, ctx VerifyContext) RecordStatus {
	var loose map[string]any
	if err := json.Unmarshal(raw, &loose); err != nil || loose == nil {
		return RecordStatus{Reason: ReasonSchemaViolation, Message: "record is not a JSON object"}
	}

	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		// Whatever survives a loose decode still identifies the record in the
		// status, so the customer can find the offending line in the package.
		s := RecordStatus{Reason: ReasonSchemaViolation, Message: "record does not match the schema: " + err.Error()}
		s.ID, _ = loose["id"].(string)
		s.Type, _ = loose["type"].(string)
		return s
	}

	reject := func(reason, format string, args ...any) RecordStatus {
		return RecordStatus{Record: r, Reason: reason, Message: fmt.Sprintf(format, args...)}
	}

	if r.Type != TypePlatform {
		return reject(ReasonUnsupportedType, "record type %q is not supported by this build", r.Type)
	}
	if !isUUID(r.ID) {
		return reject(ReasonSchemaViolation, "id %q is not a UUID", r.ID)
	}
	if rev, ok := ctx.Revoked[r.ID]; ok {
		s := reject(ReasonRevoked, "revoked at %s, reason %s", rev.RevokedAt.UTC().Format(time.RFC3339), rev.Reason)
		s.RevokedReason = rev.Reason
		return s
	}
	if r.ClusterID != ctx.ClusterID {
		return reject(ReasonClusterMismatch, "record is issued for cluster %q", r.ClusterID)
	}
	if r.ClusterKeyThumbprint != nil && *r.ClusterKeyThumbprint != ctx.ClusterKeyThumbprint {
		return reject(ReasonClusterMismatch, "record is issued for another cluster key")
	}
	if r.StartAt.IsZero() {
		return reject(ReasonSchemaViolation, "start_at is missing")
	}
	if r.ExpireAt != nil && !r.ExpireAt.After(r.StartAt) {
		return reject(ReasonSchemaViolation, "expire_at %s is not after start_at %s",
			r.ExpireAt.UTC().Format(time.RFC3339), r.StartAt.UTC().Format(time.RFC3339))
	}
	if r.GraceDays != nil && *r.GraceDays < 0 {
		return reject(ReasonSchemaViolation, "grace_days is %d, must not be negative", *r.GraceDays)
	}
	if r.Platform != nil {
		if err := requireObject("platform.expansions", r.Platform.Expansions); err != nil {
			return reject(ReasonSchemaViolation, "%s", err)
		}
		if err := requireObject("platform.extra_components", r.Platform.ExtraComponents); err != nil {
			return reject(ReasonSchemaViolation, "%s", err)
		}
		for _, name := range sortedLimitNames(r.Platform.ResourceLimits) {
			if v := r.Platform.ResourceLimits[name]; v != nil && *v < 0 {
				return reject(ReasonSchemaViolation, "resource_limits[%q] is %d, must not be negative", name, *v)
			}
		}
	}

	return RecordStatus{Record: r, Accepted: true}
}

// sortedLimitNames keeps the rejection message of a package with several
// negative limits the same on every reconcile.
func sortedLimitNames(m map[string]*int64) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// requireObject enforces the shape of a field this iteration does not
// interpret. An array or a scalar is not an unknown field, it is a known field
// of the wrong shape.
func requireObject(name string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("%s is not valid JSON", name)
	}
	if v == nil {
		return nil
	}
	if _, ok := v.(map[string]any); !ok {
		return fmt.Errorf("%s must be a JSON object", name)
	}
	return nil
}

// RecordIDsUnverified decodes the record ids of a package without checking the
// signature. It exists for input-time deduplication, where the ids are needed
// before the package is trusted; never use it for anything else.
func RecordIDsUnverified(token string) ([]string, error) {
	t, err := Parse(token)
	if err != nil {
		return nil, err
	}
	var p struct {
		Licenses []struct {
			ID string `json:"id"`
		} `json:"licenses"`
	}
	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return nil, fmt.Errorf("%w: payload is not JSON: %s", ErrMalformed, err)
	}
	ids := make([]string, 0, len(p.Licenses))
	for _, l := range p.Licenses {
		ids = append(ids, l.ID)
	}
	return ids, nil
}

// isUUID checks the canonical 8-4-4-4-12 hexadecimal form.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
