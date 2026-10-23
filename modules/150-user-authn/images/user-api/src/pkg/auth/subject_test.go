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

package auth

import (
	"encoding/base64"
	"testing"
)

func TestParseSubject(t *testing.T) {
	t.Parallel()

	encode := func(raw ...byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

	tests := []struct {
		name            string
		sub             string
		wantUserID      string
		wantConnectorID string
		wantErr         bool
	}{
		// The vectors of the MR3 contract, appendix A; Dex encodes them with tokens.GenSubject.
		{
			name:            "local user",
			sub:             "CgRqYW5lEgVsb2NhbA",
			wantUserID:      "jane",
			wantConnectorID: "local",
		},
		{
			name:            "ldap user",
			sub:             "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA",
			wantUserID:      "cn=jdoe,ou=people,dc=example,dc=org",
			wantConnectorID: "ldap",
		},
		{
			name:            "fields in reverse order",
			sub:             encode(0x12, 0x05, 'l', 'o', 'c', 'a', 'l', 0x0a, 0x04, 'j', 'a', 'n', 'e'),
			wantUserID:      "jane",
			wantConnectorID: "local",
		},
		{name: "padded base64", sub: "CgRqYW5lEgVsb2NhbA==", wantErr: true},
		{name: "standard base64 alphabet", sub: "CgRqYW5l+gVsb2NhbA", wantErr: true},
		{name: "empty", sub: "", wantErr: true},
		{name: "connector ID missing", sub: encode(0x0a, 0x04, 'j', 'a', 'n', 'e'), wantErr: true},
		{name: "user ID missing", sub: encode(0x12, 0x05, 'l', 'o', 'c', 'a', 'l'), wantErr: true},
		{name: "empty user ID", sub: encode(0x0a, 0x00, 0x12, 0x01, 'l'), wantErr: true},
		{name: "repeated user ID", sub: encode(0x0a, 0x01, 'a', 0x0a, 0x01, 'b', 0x12, 0x01, 'l'), wantErr: true},
		{name: "unexpected field", sub: encode(0x0a, 0x01, 'a', 0x12, 0x01, 'l', 0x1a, 0x01, 'x'), wantErr: true},
		{name: "varint instead of string", sub: encode(0x08, 0x01, 0x12, 0x01, 'l'), wantErr: true},
		{name: "truncated value", sub: encode(0x0a, 0x04, 'j', 'a'), wantErr: true},
		{name: "invalid UTF-8", sub: encode(0x0a, 0x01, 0xff, 0x12, 0x01, 'l'), wantErr: true},
		{name: "not base64", sub: "not a subject!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userID, connectorID, err := ParseSubject(tt.sub)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSubject(%q) = (%q, %q), want an error", tt.sub, userID, connectorID)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSubject(%q) error = %v", tt.sub, err)
			}
			if userID != tt.wantUserID || connectorID != tt.wantConnectorID {
				t.Errorf("ParseSubject(%q) = (%q, %q), want (%q, %q)",
					tt.sub, userID, connectorID, tt.wantUserID, tt.wantConnectorID)
			}
		})
	}
}

func FuzzParseSubject(f *testing.F) {
	f.Add("CgRqYW5lEgVsb2NhbA")
	f.Add("CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA")

	f.Fuzz(func(t *testing.T, sub string) {
		userID, connectorID, err := ParseSubject(sub)
		if err != nil {
			return
		}
		if userID == "" || connectorID == "" {
			t.Fatalf("ParseSubject(%q) accepted an empty field: (%q, %q)", sub, userID, connectorID)
		}
	})
}
