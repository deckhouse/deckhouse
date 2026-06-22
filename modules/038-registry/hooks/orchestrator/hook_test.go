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

package orchestrator

import "testing"

// TestShouldInitializeRequiresRestoredState asserts the restore-only rule:
// initialize runs ONLY when a prior registry-state was restored (legacy cluster),
// never on a fresh install (no registry-state, but registry-init present).
func TestShouldInitializeRequiresRestoredState(t *testing.T) {
	cases := []struct {
		name          string
		stateRestored bool
		initExists    bool
		initApplied   bool
		want          bool
	}{
		{"fresh-install-init-present", false, true, false, false},
		{"fresh-install-no-init", false, false, false, false},
		{"legacy-restored-init-unapplied", true, true, false, true},
		{"legacy-restored-init-applied", true, true, true, false},
		{"legacy-restored-no-init", true, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldInitialize(c.stateRestored, InitSecretSnap{IsExist: c.initExists, Applied: c.initApplied})
			if got != c.want {
				t.Fatalf("shouldInitialize(stateRestored=%v, init={exist:%v applied:%v}) = %v, want %v",
					c.stateRestored, c.initExists, c.initApplied, got, c.want)
			}
		})
	}
}
