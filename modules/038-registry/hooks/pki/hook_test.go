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

package pki

import "testing"

func TestShouldMarkInitApplied(t *testing.T) {
	initCA := &CertModel{Cert: "INIT-CA-CERT", Key: "INIT-CA-KEY"}

	cases := []struct {
		name            string
		fromInit        *State
		initExists      bool
		initApplied     bool
		moduleInCluster bool
		moduleSnap      State
		want            bool
	}{
		{
			name:            "persisted: init CA present in in-cluster module-pki -> mark",
			fromInit:        &State{CA: initCA},
			initExists:      true,
			moduleInCluster: true,
			moduleSnap:      State{CA: &CertModel{Cert: "INIT-CA-CERT"}},
			want:            true,
		},
		{
			name:            "init absent -> do not mark",
			fromInit:        nil,
			initExists:      false,
			moduleInCluster: true,
			moduleSnap:      State{CA: &CertModel{Cert: "INIT-CA-CERT"}},
			want:            false,
		},
		{
			name:            "already applied -> do not re-mark",
			fromInit:        &State{CA: initCA},
			initExists:      true,
			initApplied:     true,
			moduleInCluster: true,
			moduleSnap:      State{CA: &CertModel{Cert: "INIT-CA-CERT"}},
			want:            false,
		},
		{
			name:            "init CA nil -> do not mark",
			fromInit:        &State{CA: nil},
			initExists:      true,
			moduleInCluster: true,
			moduleSnap:      State{CA: &CertModel{Cert: "INIT-CA-CERT"}},
			want:            false,
		},
		{
			name:            "module-pki only in values fallback (not in cluster) -> do not mark (would lose CA)",
			fromInit:        &State{CA: initCA},
			initExists:      true,
			moduleInCluster: false,
			moduleSnap:      State{CA: &CertModel{Cert: "INIT-CA-CERT"}},
			want:            false,
		},
		{
			name:            "module-pki in cluster but no CA yet -> do not mark",
			fromInit:        &State{CA: initCA},
			initExists:      true,
			moduleInCluster: true,
			moduleSnap:      State{CA: nil},
			want:            false,
		},
		{
			name:            "module-pki CA differs from init CA -> do not mark (init CA not adopted)",
			fromInit:        &State{CA: initCA},
			initExists:      true,
			moduleInCluster: true,
			moduleSnap:      State{CA: &CertModel{Cert: "OTHER-CA-CERT"}},
			want:            false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldMarkInitApplied(tc.fromInit, tc.initExists, tc.initApplied, tc.moduleInCluster, tc.moduleSnap)
			if got != tc.want {
				t.Fatalf("shouldMarkInitApplied = %v, want %v", got, tc.want)
			}
		})
	}
}
