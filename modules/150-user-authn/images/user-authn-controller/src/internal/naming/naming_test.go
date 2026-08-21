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

package naming

import (
	"strings"
	"testing"
)

func TestToFnvLikeDex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "admin@example.com",
			input: "admin@example.com",
			want:  "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
		},
		{
			name:  "empty string is stable",
			input: "",
			want:  ToFnvLikeDex(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToFnvLikeDex(tt.input)
			if got != tt.want {
				t.Errorf("ToFnvLikeDex(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOfflineTokenNameDiffersFromConcatenatedToFnv(t *testing.T) {
	t.Parallel()

	userID := "uid-1"
	connID := "ldap-main"
	got := OfflineTokenName(userID, connID)
	concat := ToFnvLikeDex(userID + connID)
	if got == concat {
		t.Fatalf("OfflineTokenName must not equal ToFnvLikeDex(userID+connID); both %q", got)
	}
	if got == "" {
		t.Fatal("OfflineTokenName returned empty")
	}
}

func TestLocalName(t *testing.T) {
	t.Parallel()

	got := LocalName("Admin@Example.com")
	want := "local-mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf"
	if got != want {
		t.Errorf("LocalName() = %q, want %q", got, want)
	}
}

func TestExternalNameTruncatesWhenOver63(t *testing.T) {
	t.Parallel()

	connID := strings.Repeat("c", 60)
	userID := "uid"
	hash := OfflineTokenName(userID, connID)
	got := ExternalName(connID, userID)
	if got != hash {
		t.Errorf("ExternalName() = %q, want hash-only %q", got, hash)
	}
	if len(got) > maxObjectNameLength {
		t.Errorf("ExternalName() length %d exceeds %d", len(got), maxObjectNameLength)
	}
}

func TestExternalNameKeepsConnectorWhenShort(t *testing.T) {
	t.Parallel()

	got := ExternalName("ldap", "uid-1")
	want := "ldap-" + OfflineTokenName("uid-1", "ldap")
	if got != want {
		t.Errorf("ExternalName() = %q, want %q", got, want)
	}
}
