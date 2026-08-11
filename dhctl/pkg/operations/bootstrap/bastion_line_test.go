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
	"os"
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
		"kubectl --kubeconfig /tmp/dhctl/admin.kubeconfig config set-cluster kubernetes",
		"--server=https://127.0.0.1:6445",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("missing %q in %q", want, line)
		}
	}
	// A regex over the operator's only credentials: it matches a URL nobody
	// here wrote, drops a .bak beside a 0600 file, and does nothing at all when
	// it misses. kubectl edits the field by name instead.
	if strings.Contains(line, "sed") {
		t.Fatalf("the kubeconfig must not be rewritten with sed: %q", line)
	}
}

// The connect line is printed from three places, and the one that is easy to
// lose is the rerun: connectToImmutableMaster short-circuits credential
// collection when a previous attempt already saved them, so the first-collect
// print never runs, and the end-of-bootstrap print is only reached if the run
// finishes. A rerun that then stalls — waiting on a worker, say — would leave
// the operator with no idea where the kubeconfig is or how to reach a master
// that runs no sshd. Measured on a live rerun: not one of the three lines
// appeared in the log.
func TestConnectLineIsPrintedOnTheReusePath(t *testing.T) {
	src, err := os.ReadFile("steps_immutable.go")
	if err != nil {
		t.Fatalf("read steps_immutable.go: %v", err)
	}

	const reuseMarker = "reusing the admin kubeconfig at"
	idx := strings.Index(string(src), reuseMarker)
	if idx < 0 {
		t.Fatalf("the reuse branch is gone; this guard needs rewriting against whatever replaced it")
	}
	// The print has to sit inside that branch, i.e. right after the message.
	rest := string(src)[idx:]
	if end := strings.Index(rest, "\n\tserver, err :="); end > 0 {
		rest = rest[:end]
	}
	if !strings.Contains(rest, "printHowToReachTheCluster") {
		t.Fatal("a rerun reusing collected credentials must still say where they are and how to reach the master")
	}
}
