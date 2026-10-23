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

package revocation

import (
	"encoding/base64"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"user-authn-controller/internal/naming"
)

// The vectors of appendix A of the MR3 contract; Dex's tokens.GenSubject gives the same subjects.
func TestSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		userID      string
		connectorID string
		expected    string
	}{
		{name: "local user", userID: "jane", connectorID: "local", expected: "CgRqYW5lEgVsb2NhbA"},
		{
			name:        "ldap user",
			userID:      "cn=jdoe,ou=people,dc=example,dc=org",
			connectorID: "ldap",
			expected:    "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Subject(tt.userID, tt.connectorID); got != tt.expected {
				t.Errorf("Subject(%q, %q) = %q, want %q", tt.userID, tt.connectorID, got, tt.expected)
			}
		})
	}
}

// A refresh token of Dex is the protobuf message RefreshToken{refresh_id = 1, token = 2}, laid out
// as IDTokenSubject; the vector is the one of appendix A.
func TestProtoStrings_RefreshToken(t *testing.T) {
	t.Parallel()

	got := base64.RawURLEncoding.EncodeToString(protoStrings("3w4l2zgokgsohvjnqzjrv7lmz", "pbtiicr6kqrwrlqv3vn6h3pdr"))
	if expected := "ChkzdzRsMnpnb2tnc29odmpucXpqcnY3bG16EhlwYnRpaWNyNmtxcndybHF2M3ZuNmgzcGRy"; got != expected {
		t.Errorf("refresh token = %q, want %q", got, expected)
	}
}

// A user ID longer than 127 bytes takes a two-byte length.
func TestSubject_LongUserID(t *testing.T) {
	t.Parallel()

	userID := string(make([]byte, 200))
	raw, err := base64.RawURLEncoding.DecodeString(Subject(userID, "ldap"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw[0] != 0x0a || raw[1] != 0xc8 || raw[2] != 0x01 {
		t.Errorf("prefix = % x, want 0a c8 01", raw[:3])
	}
}

func TestNames(t *testing.T) {
	t.Parallel()

	if got := PairName("abcdefghijklm", 4); got != "pair-abcdefghijklm-g4" {
		t.Errorf("PairName = %q", got)
	}
	if got := SessionName("fehzwdh5qy4oyqcn7qhkcgzbm"); got != "session-fehzwdh5qy4oyqcn7qhkcgzbm" {
		t.Errorf("SessionName = %q", got)
	}
	if got := GrantName("3w4l2zgokgsohvjnqzjrv7lmz"); got != "grant-3w4l2zgokgsohvjnqzjrv7lmz" {
		t.Errorf("GrantName = %q", got)
	}
}

func TestRetention(t *testing.T) {
	t.Parallel()

	tests := []struct {
		idTokenTTL time.Duration
		expected   time.Duration
	}{
		{idTokenTTL: 10 * time.Minute, expected: 70 * time.Minute},
		{idTokenTTL: time.Minute, expected: 70 * time.Minute},
		{idTokenTTL: 5*time.Hour + 59*time.Minute, expected: 6*time.Hour + 59*time.Minute},
	}
	for _, tt := range tests {
		if got := Retention(tt.idTokenTTL); got != tt.expected {
			t.Errorf("Retention(%v) = %v, want %v", tt.idTokenTTL, got, tt.expected)
		}
	}
}

func TestObjectRoundTrip(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 10, 16, 10, 0, 0, 123456789, time.UTC)
	r := Revocation{
		Name:         PairName(naming.OfflineTokenName("jane", "local"), 3),
		Type:         TypePair,
		ConnectorID:  "local",
		UserID:       "jane",
		Subject:      Subject("jane", "local"),
		Generation:   3,
		Clients:      []string{"dex-authenticator-console@d8-console", "kubeconfig-generator-0"},
		Reason:       ReasonLock,
		Source:       SourceController,
		CreatedAt:    created,
		OperationUID: "0b7e7f0e-4b5a-4f6e-9a55-6f6a1c1d2e3f",
	}
	obj, err := r.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	if obj.GetNamespace() != naming.DexNamespace {
		t.Errorf("namespace = %q", obj.GetNamespace())
	}
	expectedLabels := map[string]string{
		TypeLabel:         TypePair,
		PairLabel:         naming.OfflineTokenName("jane", "local"),
		OperationUIDLabel: r.OperationUID,
	}
	if !reflect.DeepEqual(obj.GetLabels(), expectedLabels) {
		t.Errorf("labels = %v, want %v", obj.GetLabels(), expectedLabels)
	}
	if obj.Object["createdAt"] != "2026-10-16T10:00:00.123456789Z" {
		t.Errorf("createdAt = %v", obj.Object["createdAt"])
	}
	// Every field of the contract is there, also the empty ones: the readers do not guess.
	for _, field := range []string{"type", "connectorID", "userID", "subject", "generation", "sessionID", "grantID", "clientID", "clients", "reason", "source", "createdAt"} {
		if _, ok := obj.Object[field]; !ok {
			t.Errorf("field %s is missing", field)
		}
	}

	back, err := FromObject(obj)
	if err != nil {
		t.Fatalf("FromObject: %v", err)
	}
	if !reflect.DeepEqual(back, r) {
		t.Errorf("round trip = %+v, want %+v", back, r)
	}
}

func TestTimePrefersTheCreationTimestamp(t *testing.T) {
	t.Parallel()

	written := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	r := Revocation{Name: "x", Type: TypeGrant, CreatedAt: written}
	obj, err := r.Object()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := FromObject(obj); !got.Time().Equal(written) {
		t.Errorf("Time without a creation timestamp = %v, want the writer's %v", got.Time(), written)
	}
	server := written.Add(2 * time.Second)
	obj.SetCreationTimestamp(metav1.NewTime(server))
	got, err := FromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Time().Equal(server) || !got.Expires(time.Hour).Equal(server.Add(time.Hour)) {
		t.Errorf("Time = %v, Expires = %v; want the creation timestamp", got.Time(), got.Expires(time.Hour))
	}
}

func TestFromObjectRejectsANegativeGeneration(t *testing.T) {
	t.Parallel()

	obj, err := Revocation{Name: "x", Type: TypePair}.Object()
	if err != nil {
		t.Fatal(err)
	}
	obj.Object["generation"] = int64(-1)
	if _, err := FromObject(obj); err == nil {
		t.Fatal("FromObject accepted a negative generation")
	}
}

func TestSortedClients(t *testing.T) {
	t.Parallel()

	got := SortedClients([]string{"b", "", "a", "b"})
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("SortedClients = %v", got)
	}
}
