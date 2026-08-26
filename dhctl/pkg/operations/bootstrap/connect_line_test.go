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

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

// The line has to be usable as printed: an operator copies it, and a wrong
// port or a missing quote costs them the debugging session the line exists to
// prevent.
func TestBastionTunnelCommandShape(t *testing.T) {
	cfg := &sshconfig.Config{BastionUser: "ubuntu", BastionHost: "198.51.100.7"}
	line := bastionTunnelCommand(cfg)
	t.Logf("line: %s", line)

	if line != "ssh -f -N -D 1080 ubuntu@198.51.100.7" {
		t.Fatalf("tunnel command = %q", line)
	}
	// The tunnel outlives the command that opened it, so it must not be glued to
	// a kubectl call: printed as one line it reads as a prelude to every call,
	// and an operator following it literally collects a background ssh per call.
	for _, forbidden := range []string{"kubectl", "&&", "sed", "set-cluster", "--server="} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("the tunnel is its own step and rewrites nothing, found %q in %q", forbidden, line)
		}
	}
}

// The connect line is printed from three places, and the rerun is the easy one to
// lose: reuseCollectedKubeconfig short-circuits collection, so a stalled rerun
// would say nothing at all (observed on a live rerun: none of the three lines).
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
	// The print has to sit inside that branch, i.e. between the message and the
	// end of the function carrying it. A delimiter that stops matching would
	// quietly widen the search to the rest of the file, so its loss fails the guard.
	rest := string(src)[idx:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("the function holding the reuse branch no longer ends; this guard needs a new delimiter")
	}
	rest = rest[:end]
	if !strings.Contains(rest, "printHowToReachTheCluster") {
		t.Fatal("a rerun reusing collected credentials must still say where they are and how to reach the master")
	}
}

// An untagged Info record is file-only on a terminal: on a live bootstrap all four
// lines sat in the log while the screen showed nothing. The tags put them back on
// the screen, so they are worth a guard of their own.
func TestConnectLinesAreTaggedForTheTerminal(t *testing.T) {
	src, err := os.ReadFile("connect_line.go")
	if err != nil {
		t.Fatalf("read connect_line.go: %v", err)
	}
	body := string(src)

	start := strings.Index(body, "func (b *ClusterBootstrapper) printHowToReachTheCluster")
	if start < 0 {
		t.Fatal("printHowToReachTheCluster is gone; this guard needs rewriting")
	}
	end := strings.Index(body[start:], "\n}\n")
	if end < 0 {
		t.Fatal("cannot delimit printHowToReachTheCluster")
	}
	fn := body[start : start+end]

	// Every message the operator has to read must carry a tag.
	for _, line := range strings.Split(fn, "\n") {
		if !strings.Contains(line, "InfoContext(ctx,") {
			continue
		}
		// The call spans lines; the tag may sit on the continuation. Check the
		// statement as a whole by looking ahead to the closing parenthesis.
		idx := strings.Index(fn, line)
		stmt := fn[idx:]
		if cut := strings.Index(stmt, ")\n"); cut > 0 {
			stmt = stmt[:cut]
		}
		if !strings.Contains(stmt, "ShowInCompacted()") && !strings.Contains(stmt, "ConnectionString()") {
			t.Fatalf("this line would be file-only, so the operator never sees it: %s", strings.TrimSpace(line))
		}
	}

	// The tunnel command specifically: ConnectionString pins it as a milestone
	// and repeats it in the closing summary.
	if !strings.Contains(fn, "dhlog.ConnectionString()") {
		t.Fatal("the tunnel command must be tagged ConnectionString so it survives to the closing summary")
	}
}
