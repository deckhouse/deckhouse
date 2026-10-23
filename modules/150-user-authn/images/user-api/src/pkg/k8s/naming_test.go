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

package k8s

import (
	"strings"
	"testing"
)

// The expected names are those user-authn-controller gives: a name user-api computes differently
// is a UserAccount it never finds.
func TestAccountNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "local user by email, case-insensitive",
			got:      LocalAccountName("Admin@Example.com", "admin"),
			expected: "local-mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
		},
		{
			name:     "local user without an email",
			got:      LocalAccountName("", "admin@example.com"),
			expected: "local-mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
		},
		{
			name:     "external user",
			got:      ExternalAccountName("dex-e2e-ldap", "cn=upg-ldap-auto,ou=People,dc=example,dc=org"),
			expected: "dex-e2e-ldap-mzyoq3nkja3ik",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.expected {
				t.Errorf("name = %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

func TestExternalAccountNameOfALongConnectorIsTheHash(t *testing.T) {
	t.Parallel()

	got := ExternalAccountName(strings.Repeat("c", 60), "uid")
	if len(got) > maxObjectNameLength || strings.Contains(got, "-") {
		t.Errorf("name = %q, want the hash alone", got)
	}
}
