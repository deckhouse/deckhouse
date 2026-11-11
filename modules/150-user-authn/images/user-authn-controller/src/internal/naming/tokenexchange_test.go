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

import "testing"

func TestTokenExchangeConnectorID(t *testing.T) {
	t.Parallel()

	const uid = "1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"
	first := TokenExchangeConnectorID("gitlab-ci", uid, 0)
	if first != "tep-gitlab-ci-1a2b3c4d" {
		t.Errorf("first ID = %q, want tep-gitlab-ci-1a2b3c4d", first)
	}
	next := TokenExchangeConnectorID("gitlab-ci", uid, 1)
	if next == first || len(next) != len(first) {
		t.Errorf("second ID = %q, want another ID of the length of %q", next, first)
	}

	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "first ID", id: first, want: true},
		{name: "later ID", id: TokenExchangeConnectorID("gitlab-ci", uid, MaxTokenExchangeConnectorIDs-1), want: true},
		{name: "ID past the bound", id: TokenExchangeConnectorID("gitlab-ci", uid, MaxTokenExchangeConnectorIDs), want: false},
		{name: "ID of another provider", id: TokenExchangeConnectorID("github", uid, 0), want: false},
		{name: "ID of another UID", id: TokenExchangeConnectorID("gitlab-ci", "9f8e7d6c", 0), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsTokenExchangeConnectorID("gitlab-ci", uid, tt.id); got != tt.want {
				t.Errorf("IsTokenExchangeConnectorID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
