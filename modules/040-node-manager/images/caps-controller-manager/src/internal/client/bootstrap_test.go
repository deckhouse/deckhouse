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

package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRequestedNodeNameCommandLeavesTheNameWhereBootstrapLooksForIt(t *testing.T) {
	if got := requestedNodeNameCommand(""); got != "" {
		t.Fatalf("an unset name should add nothing to the bootstrap command, got %q", got)
	}

	got := requestedNodeNameCommand("worker-rack3-07")
	want := " && echo 'worker-rack3-07' > /var/lib/bashible/node-name"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The CRD pattern keeps anything but an RFC 1123 subdomain out of spec.nodeName,
// so this only ever runs on a caller that got past it. What the quoting has to
// guarantee is that whatever the value holds stays one shell word: the fragment
// is interpolated into a command run over SSH as root.
func TestRequestedNodeNameCommandKeepsTheNameOneShellWord(t *testing.T) {
	// One directory for the whole table, and files named by index: a per-subtest
	// TempDir would carry the case into the path, and the cases are chosen to be
	// exactly the strings a shell would rather not be handed.
	dir := t.TempDir()

	names := []string{
		"plain-name",
		"a'; touch /tmp/pwned; echo 'b",
		"quotes'\"and$backticks`",
		"$(id -u)",
		"two\nlines",
	}

	for i, name := range names {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			target := filepath.Join(dir, strconv.Itoa(i))

			script := strings.TrimPrefix(requestedNodeNameCommand(name), " && ")
			script = strings.Replace(script, "/var/lib/bashible/node-name", target, 1)

			if out, err := exec.Command("sh", "-c", script).CombinedOutput(); err != nil {
				t.Fatalf("run %q: %v (%s)", script, err, out)
			}

			written, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if got := strings.TrimSuffix(string(written), "\n"); got != name {
				t.Fatalf("the shell saw %q, not the name %q", got, name)
			}
		})
	}
}
