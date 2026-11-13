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
	"os"
	"runtime/debug"
)

// A render of a module version allocates about 150 MB (measured 2026-11-04
// on 15 modules, internal/docs.BenchmarkRenderVersions), and with GOGC=100 a
// third of it was garbage collection. GOGC=400 renders a third faster for
// about 200 MiB more memory at the peak (632 MiB against 454 MiB).
// GOMEMLIMIT, which the deployments set below the memory limit, keeps the
// heap under it whatever GOGC is, a build of the main site included: the
// collector works harder near the limit. GOGC in the environment wins.
const gcPercent = 400

func tuneGC() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(gcPercent)
	}
}
