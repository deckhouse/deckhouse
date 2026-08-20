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

package sharedgateway

import "testing"

func TestResolveFollowsTheHelmHelper(t *testing.T) {
	t.Parallel()

	module := &Ref{Namespace: "d8-module", Name: "module"}
	global := &Ref{Namespace: "infra", Name: "shared"}
	discovered := &Ref{Namespace: "d8-alb", Name: "default"}

	cases := []struct {
		name                       string
		module, global, discovered *Ref
		want                       string
	}{
		{
			name: "nothing names a gateway",
			want: "",
		},
		{
			name:       "only the discovered one",
			discovered: discovered,
			want:       "d8-alb/default",
		},
		{
			name:       "the operator named one, which wins over the discovered one",
			global:     global,
			discovered: discovered,
			want:       "infra/shared",
		},
		{
			name:       "the module named one, which wins over both",
			module:     module,
			global:     global,
			discovered: discovered,
			want:       "d8-module/module",
		},
		{
			// The helper picks by which key is present and checks the halves afterwards, so a
			// reference written with one half shadows the gateway that would have been found
			// otherwise. The platform attaches its own routes to nothing in that cluster, so
			// nothing is exempt from the reservation either.
			name:       "a reference with no namespace, which names no gateway at all",
			global:     &Ref{Name: "shared"},
			discovered: discovered,
			want:       "",
		},
		{
			name:       "a reference with no name",
			global:     &Ref{Namespace: "infra"},
			discovered: discovered,
			want:       "",
		},
		{
			// What global-hooks/discovery/default_gateway.go writes when it finds no gateway to
			// discover, which is every cluster without the d8-alb ConfigMap.
			name:       "a discovered reference with both halves empty",
			discovered: &Ref{},
			want:       "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := Resolve(tc.module, tc.global, tc.discovered).String(); got != tc.want {
				t.Errorf("Resolve(%v, %v, %v) = %q, want %q", tc.module, tc.global, tc.discovered, got, tc.want)
			}
		})
	}
}

func TestIsAnswersForOneObject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		ref               Ref
		namespace, object string
		want              bool
	}{
		{
			name:      "the object it names",
			ref:       Ref{Namespace: "infra", Name: "shared"},
			namespace: "infra",
			object:    "shared",
			want:      true,
		},
		{
			name:      "the same name in another namespace",
			ref:       Ref{Namespace: "infra", Name: "shared"},
			namespace: "tenant",
			object:    "shared",
		},
		{
			name:      "another name in the same namespace",
			ref:       Ref{Namespace: "infra", Name: "shared"},
			namespace: "infra",
			object:    "other",
		},
		{
			// Nothing is exempt where no gateway is named, and least of all an object whose
			// namespace and name happen to be as empty as the reference.
			name: "no reference at all",
			ref:  Ref{},
		},
		{
			name:   "half a reference and an object that matches the half",
			ref:    Ref{Name: "shared"},
			object: "shared",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.ref.Is(tc.namespace, tc.object); got != tc.want {
				t.Errorf("Ref%+v.Is(%q, %q) = %v, want %v", tc.ref, tc.namespace, tc.object, got, tc.want)
			}
		})
	}
}
