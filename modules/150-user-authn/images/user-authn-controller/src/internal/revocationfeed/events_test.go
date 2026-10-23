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

package revocationfeed

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"user-authn-controller/internal/revocation"
)

var vectorSalt = Salt([]byte("s3cr3t"))

// The event format is the contract with dex-authenticator: field names, order, and what is left
// out. These are the exact bytes of the data lines.
func TestEvents_Golden(t *testing.T) {
	t.Parallel()

	at := time.Unix(1760610000, 0).UTC()
	tests := []struct {
		name     string
		rev      revocation.Revocation
		expected string
	}{
		{
			name:     "pair",
			rev:      revocation.Revocation{Type: revocation.TypePair, Subject: "CgRqYW5lEgVsb2NhbA", Generation: 3, Created: at},
			expected: `{"v":1,"type":"pair","k":"BtOeixcwfXZ3z6kDWHNMTw","g":3,"exp":1760614200,"at":1760610000}`,
		},
		{
			name:     "session",
			rev:      revocation.Revocation{Type: revocation.TypeSession, SessionID: "fehzwdh5qy4oyqcn7qhkcgzbm", Created: at},
			expected: `{"v":1,"type":"session","k":"0-SQllhFkuuG1ttZoMxAiA","exp":1760614200,"at":1760610000}`,
		},
		{
			name:     "grant",
			rev:      revocation.Revocation{Type: revocation.TypeGrant, GrantID: "3w4l2zgokgsohvjnqzjrv7lmz", Created: at},
			expected: `{"v":1,"type":"grant","k":"jVFqq8vBRSjPWcgLzpRzaA","exp":1760614200,"at":1760610000}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec, ok := recordOf(tt.rev, retention)
			if !ok {
				t.Fatal("recordOf refused the revocation")
			}
			data, err := revocationEvent(rec, vectorSalt)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.expected {
				t.Errorf("data = %s\nwant   %s", data, tt.expected)
			}
		})
	}

	heartbeat, err := heartbeatEvent(time.Unix(1760610010, 0))
	if err != nil {
		t.Fatal(err)
	}
	if string(heartbeat) != `{"v":1,"t":1760610010}` {
		t.Errorf("heartbeat = %s", heartbeat)
	}
}

func TestSnapshotParts_Golden(t *testing.T) {
	t.Parallel()

	at := time.Unix(1760610000, 0).UTC()
	revs := []revocation.Revocation{
		{Type: revocation.TypePair, Subject: "CgRqYW5lEgVsb2NhbA", Generation: 2, Created: at.Add(-time.Minute)},
		{Type: revocation.TypePair, Subject: "CgRqYW5lEgVsb2NhbA", Generation: 3, Created: at},
		{Type: revocation.TypeSession, SessionID: "fehzwdh5qy4oyqcn7qhkcgzbm", Created: at},
		{Type: revocation.TypeGrant, GrantID: "3w4l2zgokgsohvjnqzjrv7lmz", Created: at},
		// Expired: left out.
		{Type: revocation.TypeGrant, GrantID: "old", Created: at.Add(-2 * time.Hour)},
		// Unknown type: left out.
		{Type: "device", GrantID: "x", Created: at},
	}
	parts, err := snapshotParts(revs, vectorSalt, retention, at, 512)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"v":1,"part":0,"last":true,"retention":4200,` +
		`"pairs":[{"k":"BtOeixcwfXZ3z6kDWHNMTw","g":3,"exp":1760614200}],` +
		`"sessions":[{"k":"0-SQllhFkuuG1ttZoMxAiA","exp":1760614200}],` +
		`"grants":[{"k":"jVFqq8vBRSjPWcgLzpRzaA","exp":1760614200}]}`
	if len(parts) != 1 || string(parts[0]) != expected {
		t.Errorf("parts = %s\nwant    %s", parts, expected)
	}
}

func TestSnapshotParts_Empty(t *testing.T) {
	t.Parallel()

	parts, err := snapshotParts(nil, vectorSalt, retention, feedNow, 512)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"v":1,"part":0,"last":true,"retention":4200,"pairs":[],"sessions":[],"grants":[]}`
	if len(parts) != 1 || string(parts[0]) != expected {
		t.Errorf("parts = %s, want one empty part", parts)
	}
}

func TestSnapshotParts_Split(t *testing.T) {
	t.Parallel()

	var revs []revocation.Revocation
	for i := range 5 {
		revs = append(revs, revocation.Revocation{Type: revocation.TypeGrant, GrantID: fmt.Sprintf("grant-%d", i), Created: feedNow})
	}
	parts, err := snapshotParts(revs, vectorSalt, retention, feedNow, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(parts))
	}
	for i, part := range parts {
		data := decode[snapshotData](t, string(part))
		if data.Part != i || data.Last != (i == 2) {
			t.Errorf("part %d = %+v", i, data)
		}
		if !strings.Contains(string(part), `"pairs":[]`) {
			t.Errorf("part %d has no empty pairs: %s", i, part)
		}
	}
}
