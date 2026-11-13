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

package main

import (
	"runtime/debug"
	"testing"
)

func TestTuneGC(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(100))

	t.Setenv("GOGC", "")
	tuneGC()
	if got := debug.SetGCPercent(100); got != gcPercent {
		t.Errorf("GOGC %d, want %d", got, gcPercent)
	}

	// GOGC in the environment wins.
	t.Setenv("GOGC", "150")
	tuneGC()
	if got := debug.SetGCPercent(100); got != 100 {
		t.Errorf("GOGC %d after an environment GOGC, want it untouched", got)
	}
}
