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

package provider

import (
	"encoding/base64"
	"testing"
)

func TestIDTokenSubject(t *testing.T) {
	t.Parallel()

	token := func(payload string) string {
		return "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".c2lnbmF0dXJl"
	}

	tests := []struct {
		name           string
		raw            string
		wantSubject    string
		wantGeneration uint64
		wantErr        bool
	}{
		{
			name:           "token with a generation",
			raw:            token(`{"sub":"CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA","d8_gen":3,"groups":["admins"]}`),
			wantSubject:    "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA",
			wantGeneration: 3,
		},
		{
			// Dex leaves the claim out at generation 0, and so did every token before MR3.
			name:        "token without a generation",
			raw:         token(`{"sub":"CgRqYW5lEgVsb2NhbA"}`),
			wantSubject: "CgRqYW5lEgVsb2NhbA",
		},
		{name: "no ID token", raw: "", wantErr: true},
		{name: "not a JWT", raw: "opaque-token", wantErr: true},
		{name: "payload not base64", raw: "a.%%%.c", wantErr: true},
		{name: "payload not JSON", raw: token(`not json`), wantErr: true},
		{name: "no subject", raw: token(`{"d8_gen":3}`), wantErr: true},
		{name: "negative generation", raw: token(`{"sub":"x","d8_gen":-1}`), wantErr: true},
		{name: "fractional generation", raw: token(`{"sub":"x","d8_gen":1.5}`), wantErr: true},
		{name: "generation as a string", raw: token(`{"sub":"x","d8_gen":"3"}`), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			subject, generation, err := idTokenSubject(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("idTokenSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
			if subject != tt.wantSubject || generation != tt.wantGeneration {
				t.Errorf("idTokenSubject() = (%q, %d), want (%q, %d)", subject, generation, tt.wantSubject, tt.wantGeneration)
			}
		})
	}
}
