//go:build !integration

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

package waypointcontroller

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestOwnerUIDs(t *testing.T) {
	cases := []struct {
		name string
		obj  appsv1.Deployment
		want []string
	}{
		{
			name: "no_owners",
			obj:  appsv1.Deployment{},
			want: nil,
		},
		{
			name: "single_owner",
			obj: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{UID: types.UID("abc-123")},
					},
				},
			},
			want: []string{"abc-123"},
		},
		{
			name: "skip_empty_uid",
			obj: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{UID: ""},
						{UID: types.UID("real-uid")},
					},
				},
			},
			want: []string{"real-uid"},
		},
		{
			name: "multiple_owners",
			obj: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{UID: types.UID("abc-123")},
						{UID: types.UID("def-456")},
						{UID: types.UID("ghi-789")},
					},
				},
			},
			want: []string{"abc-123", "def-456", "ghi-789"},
		},
		{
			name: "all_empty_uids",
			obj: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{UID: ""},
						{UID: ""},
					},
				},
			},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ownerUIDs(&tc.obj)

			if len(got) != len(tc.want) {
				t.Errorf("ownerUIDs() len = %d, want %d; got = %v", len(got), len(tc.want), got)
				return
			}

			for i, want := range tc.want {
				if got[i] != want {
					t.Errorf("ownerUIDs()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
