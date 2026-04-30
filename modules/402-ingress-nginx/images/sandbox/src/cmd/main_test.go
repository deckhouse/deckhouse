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

package main

import (
	"reflect"
	"testing"
)

func TestNormalizeSandboxArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "drops leading separator",
			in:   []string{"--", "/usr/bin/unshare", "-R", "/validation-chroot"},
			want: []string{"/usr/bin/unshare", "-R", "/validation-chroot"},
		},
		{
			name: "keeps plain argv intact",
			in:   []string{"/usr/bin/unshare", "-R", "/validation-chroot"},
			want: []string{"/usr/bin/unshare", "-R", "/validation-chroot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSandboxArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected argv, got %v want %v", got, tt.want)
			}
		})
	}
}
