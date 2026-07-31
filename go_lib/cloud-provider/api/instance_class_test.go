// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"regexp"
	"strings"
	"testing"
)

func TestBuildInstanceClassName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		nodeGroupName string
		want          string
	}{
		{
			name:          "master node group",
			nodeGroupName: "master",
			want:          "master-fc613b4dfd67",
		},
		{
			name:          "worker node group",
			nodeGroupName: "worker",
			want:          "worker-87eba76e7f31",
		},
		{
			name:          "long node group name is truncated before hash suffix",
			nodeGroupName: "very-long-node-group-name-with-many-segments-and-truncation-case",
			want:          "very-long-node-group-name-with-many-segments-and-t-5f6b2322082d",
		},
		{
			name:          "truncated prefix does not end with dash",
			nodeGroupName: strings.Repeat("a", 49) + "-tail",
			want:          strings.Repeat("a", 49) + "-06def65dd43b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := BuildInstanceClassName(tt.nodeGroupName)
			if got != tt.want {
				t.Fatalf("BuildInstanceClassName(%q) = %q, want %q", tt.nodeGroupName, got, tt.want)
			}
		})
	}
}

func TestBuildInstanceClassNameDNSLabel(t *testing.T) {
	t.Parallel()

	got := BuildInstanceClassName("very-long-node-group-name-with-many-segments-and-truncation-case")
	if len(got) > 63 {
		t.Fatalf("BuildInstanceClassName() length = %d, want <= 63", len(got))
	}

	dnsLabel := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !dnsLabel.MatchString(got) {
		t.Fatalf("BuildInstanceClassName() = %q, want DNS-1123 label", got)
	}
}

func TestBuildInstanceClassNameDeterministicAndDistinct(t *testing.T) {
	t.Parallel()

	first := BuildInstanceClassName("worker")
	second := BuildInstanceClassName("worker")
	if first != second {
		t.Fatalf("BuildInstanceClassName() is not deterministic: %q != %q", first, second)
	}

	other := BuildInstanceClassName("worker-extra")
	if first == other {
		t.Fatalf("BuildInstanceClassName() returned same name for distinct node groups: %q", first)
	}
}
