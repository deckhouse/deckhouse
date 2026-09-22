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
	"testing"
	"time"
)

const (
	testClusterID  = "7f3c1a94-2b6e-4d51-9c08-a5e7d2f81b30"
	otherClusterID = "11111111-2222-3333-4444-555555555555"
	recordA        = "c0d100e0-4ed0-4da4-9742-a1b2c3d4e5f6"
	recordB        = "1a7f93b2-0c58-4d61-8e30-9f4a6b2c8d17"
	recordC        = "9c4e12ab-7f30-4d88-b512-6a0e3d7c9f21"
	testPackageJTI = "00000000-0000-4000-8000-00000000abcd"
)

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

// issue signs an arbitrary payload as a license package.
func issue(t *testing.T, priv ed25519.PrivateKey, payload any) string {
	t.Helper()
	token, err := Sign(map[string]any{"typ": TypLicense}, payload, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

// pkgOf wraps records into a minimal valid package payload.
func pkgOf(records ...any) map[string]any {
	return map[string]any{
		"ver":           1,
		"iss":           Issuer,
		"sub":           "9bc51496-5737-4144-87d8-e5f6a7b8b9c0",
		"jti":           "9c4e12ab-7f30-4d88-b512-6a0e3d7c9f21",
		"iat":           "2026-01-01T00:00:00Z",
		"customer_name": "Acme",
		"licenses":      records,
	}
}

// platformOf builds a Platform record for the test cluster.
func platformOf(id string, platform map[string]any) map[string]any {
	return map[string]any{
		"type":       TypePlatform,
		"id":         id,
		"start_at":   "2026-01-01T00:00:00Z",
		"expire_at":  "2026-07-01T00:00:00Z",
		"cluster_id": testClusterID,
		"origin":     "purchase",
		"platform":   platform,
	}
}

func platformLimits(limits map[string]any) map[string]any {
	return map[string]any{"edition": "Core", "resource_limits": limits}
}

func ctxFor(pub ed25519.PublicKey) VerifyContext {
	return VerifyContext{
		VendorKeys: []ed25519.PublicKey{pub},
		ClusterID:  testClusterID,
		Now:        ts("2026-02-01T00:00:00Z"),
	}
}

func ts(s string) time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return v
}

func tsp(s string) *time.Time {
	v := ts(s)
	return &v
}

func i64(v int64) *int64 { return &v }

// wl builds an already verified Platform record status, the input shape of
// Compute. expire may be empty for a perpetual record.
func wl(id, start, expire string, limits map[string]*int64) RecordStatus {
	r := Record{
		Type:      TypePlatform,
		ID:        id,
		StartAt:   ts(start),
		ClusterID: testClusterID,
	}
	if expire != "" {
		r.ExpireAt = tsp(expire)
	}
	if limits != nil {
		r.Platform = &Platform{Edition: "Core", ResourceLimits: limits}
	}
	return RecordStatus{Record: r, Accepted: true}
}

func oneKey(records ...RecordStatus) []KeyRecords {
	return []KeyRecords{{Key: "license-1", JTI: testPackageJTI, Records: records}}
}

// input builds a Compute input with the defaults every test shares. Fields that
// a particular test cares about are set on the returned value.
func input(now time.Time, keys []KeyRecords, nodes ...Node) Input {
	return Input{Keys: keys, Nodes: nodes, Now: now, Thresholds: DefaultThresholds()}
}

// nd is one licensable node of a given size.
func nd(name string, vcpu int64) Node { return Node{Name: name, VCPU: vcpu} }

// nodeNames flattens an allocation group for comparison.
func nodeNames(nodes []Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}

// rejectedRec builds a record that did not pass the record level checks.
func rejectedRec(id, reason string) RecordStatus {
	r := wl(id, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{MetricVCPU: i64(100)})
	r.Accepted = false
	r.Reason = reason
	return r
}

func statusOf(t *testing.T, res Result, id string) RecordStatus {
	t.Helper()
	for _, r := range res.Records {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("record %q not found", id)
	return RecordStatus{}
}
