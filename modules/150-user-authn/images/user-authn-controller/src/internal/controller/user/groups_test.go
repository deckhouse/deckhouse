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

package user

import (
	"slices"
	"testing"
)

func TestGroupsForUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []groupView
		user   string
		want   []string
	}{
		{
			name: "nested groups",
			groups: []groupView{
				{SpecName: "group-1", Members: []groupMember{{Kind: memberKindUser, Name: "admin"}}},
				{SpecName: "group-2", Members: []groupMember{{Kind: memberKindGroup, Name: "group-1"}}},
				{SpecName: "group-3", Members: []groupMember{{Kind: memberKindUser, Name: "admin"}}},
			},
			user: "admin",
			want: []string{"group-1", "group-2", "group-3"},
		},
		{
			name: "cycle does not loop",
			groups: []groupView{
				{SpecName: "a", Members: []groupMember{{Kind: memberKindGroup, Name: "b"}, {Kind: memberKindUser, Name: "admin"}}},
				{SpecName: "b", Members: []groupMember{{Kind: memberKindGroup, Name: "a"}}},
			},
			user: "admin",
			want: []string{"a", "b"},
		},
		{
			name:   "no groups",
			groups: nil,
			user:   "admin",
			want:   []string{},
		},
		{
			name: "user not a member",
			groups: []groupView{
				{SpecName: "gods", Members: []groupMember{{Kind: memberKindUser, Name: "Athena"}}},
			},
			user: "admin",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := groupsForUser(tt.groups, tt.user)
			if !slices.Equal(got, tt.want) {
				t.Errorf("groupsForUser = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUsersInGroupSubtree(t *testing.T) {
	t.Parallel()

	groups := []groupView{
		{SpecName: "group-1", Members: []groupMember{{Kind: memberKindUser, Name: "admin"}}},
		{SpecName: "group-2", Members: []groupMember{{Kind: memberKindGroup, Name: "group-1"}}},
	}
	got := usersInGroupSubtree(groups, "group-2")
	slices.Sort(got)
	if !slices.Equal(got, []string{"admin"}) {
		t.Errorf("usersInGroupSubtree(group-2) = %v, want [admin]", got)
	}
	if got := usersInGroupSubtree(groups, ""); got != nil {
		t.Errorf("empty spec name = %v, want nil", got)
	}
}

func TestEqualStringSets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{name: "nil and empty", a: nil, b: []string{}, want: true},
		{name: "same order", a: []string{"a", "b"}, b: []string{"a", "b"}, want: true},
		{name: "different order", a: []string{"b", "a"}, b: []string{"a", "b"}, want: true},
		{name: "mismatch", a: []string{"a", "b"}, b: []string{"a", "c"}, want: false},
		{name: "duplicate vs distinct", a: []string{"x", "y"}, b: []string{"x", "x"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := equalStringSets(tt.a, tt.b); got != tt.want {
				t.Errorf("equalStringSets(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
