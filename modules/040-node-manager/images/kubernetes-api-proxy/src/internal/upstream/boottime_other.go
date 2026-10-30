//go:build !linux

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

package upstream

import "time"

var processStart = time.Now()

// bootMillis returns the time since the process start in milliseconds. The
// proxy runs on Linux only; this version is for local builds and tests.
func bootMillis() uint64 {
	return uint64(time.Since(processStart).Milliseconds())
}
