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

package bootstrap

import (
	"strings"
	"testing"
)

// The line has to be usable as printed: an operator copies it, and a wrong
// port or a missing quote costs them the debugging session the line exists to
// prevent.
func TestBastionForwardLineShape(t *testing.T) {
	line := buildBastionForwardLine("ubuntu", "198.51.100.7", 0, "192.0.2.10", "/tmp/dhctl/admin.kubeconfig")
	t.Logf("line: %s", line)
	for _, want := range []string{
		"ssh -f -N -L 6445:192.0.2.10:6443 ubuntu@198.51.100.7",
		"https://192.0.2.10:6443",
		"https://127.0.0.1:6445",
		"/tmp/dhctl/admin.kubeconfig",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("missing %q in %q", want, line)
		}
	}
}
