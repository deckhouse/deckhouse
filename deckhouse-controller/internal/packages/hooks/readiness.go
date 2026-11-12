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

package hooks

import "strings"

// readinessSuffix ends the name of a package's readiness hook. Batch hook names carry their
// binding key after a colon, so no other hook name ends with it.
const readinessSuffix = "-readiness"

// ReadinessHookName is the name of the readiness hook the hook binary named hook declares.
func ReadinessHookName(hook string) string {
	return hook + readinessSuffix
}

// IsReadinessHook reports whether name is the name of a readiness hook (ReadinessHookName).
func IsReadinessHook(name string) bool {
	return strings.HasSuffix(name, readinessSuffix)
}

// ProbeFailedError is a readiness probe that found the package not ready. A readiness hook of an
// SDK that reports through the Module's IsReady condition gets it as the error of its run, so the
// hook lanes read every probe verdict from the run's outcome.
type ProbeFailedError struct {
	Message string
}

func (e *ProbeFailedError) Error() string {
	return "readiness probe failed: " + e.Message
}
