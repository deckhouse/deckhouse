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

package template

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The preflight scripts are the part of the checks the Go tests cannot see: the checks are tested
// against what a script printed, never against whether the script prints it. These run the real
// scripts, rendered from the real templates, with the commands they consult stubbed on PATH — the
// same thing a bats suite would do, in the test runner the repository already has, so it runs in
// CI rather than only on a machine where bats happens to be installed.

// candiDir is the checkout's candi directory, where the templates live.
func candiDir(t *testing.T) string {
	t.Helper()

	// pkg/template -> dhctl -> the checkout root.
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "candi"))
	require.NoError(t, err)

	if _, err := os.Stat(dir); err != nil {
		t.Skipf("candi directory not found at %s", dir)
	}
	return dir
}

// renderPreflightScript renders one of the preflight templates to a runnable file.
func renderPreflightScript(t *testing.T, name string) string {
	t.Helper()

	source := filepath.Join(candiDir(t), "bashible", "preflight", name+".tpl")

	// The name is the temp-file pattern, not a path: the rendered script lands wherever the
	// production code puts it, which is what the checks upload.
	rendered, err := RenderAndSaveTemplate(context.Background(), name, source, map[string]any{})
	require.NoError(t, err, "rendering %s", source)
	t.Cleanup(func() { _ = os.Remove(rendered) })

	require.NoError(t, os.Chmod(rendered, 0o755))
	return rendered
}

// stub writes a fake command onto a directory that will be prepended to PATH. body is the whole
// script after the shebang.
type stub struct {
	name string
	body string
}

// passthrough makes stubs that hand a command straight through to the real one. The scripts use
// ordinary utilities as well as the commands under test, and those are not what is being faked.
func passthrough(t *testing.T, names ...string) []stub {
	t.Helper()

	stubs := make([]stub, 0, len(names))
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not available on this host", name)
		}
		stubs = append(stubs, stub{name: name, body: `exec ` + path + ` "$@"`})
	}
	return stubs
}

// runScript runs the script with the stubs as its entire PATH. Isolating it that way is the point:
// a command the script reaches for that the test did not account for is a dependency on the host
// running the test, and it fails loudly instead of quietly passing because this machine happens to
// have it. Returning the exit status rather than an error keeps the assertions on what the node's
// operator sees.
func runScript(t *testing.T, script string, stubs ...stub) (output string, exitCode int) {
	t.Helper()

	return runScriptWithEnv(t, script, nil, stubs...)
}

// runScriptWithEnv is runScript with extra environment for the stubs to read.
func runScriptWithEnv(t *testing.T, script string, env []string, stubs ...stub) (output string, exitCode int) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the node scripts are bash")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available on this host")
	}

	stubDir := t.TempDir()
	for _, s := range stubs {
		path := filepath.Join(stubDir, s.name)
		// /bin/sh, not env bash: PATH holds nothing but the stubs, so a stub cannot look
		// up its own interpreter.
		require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+s.body+"\n"), 0o755))
	}

	cmd := exec.Command(bash, script)
	cmd.Env = append(os.Environ(), "PATH="+stubDir)
	cmd.Env = append(cmd.Env, env...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			require.NoError(t, err)
		}
		return string(out), exitErr.ExitCode()
	}

	return string(out), 0
}

// TestCheckLocalhostScript: an /etc/hosts without a localhost line breaks the control plane in
// ways that look like anything but that, which is the whole reason this check exists.
func TestCheckLocalhostScript(t *testing.T) {
	script := renderPreflightScript(t, "check_localhost.sh")

	tests := []struct {
		name string
		// getent is the body of the stubbed getent.
		getent   string
		wantCode int
		wantOut  string
	}{
		{
			name:     "localhost resolves to 127.0.0.1",
			getent:   `echo "127.0.0.1       STREAM localhost"; echo "127.0.0.1       DGRAM"`,
			wantCode: 0,
		},
		{
			// The common breakage: only the v6 address is mapped, and everything that
			// dials 127.0.0.1 by name fails.
			name:     "only ::1 is mapped",
			getent:   `echo "::1             STREAM localhost"`,
			wantCode: 1,
			wantOut:  "does not resolve",
		},
		{
			name:     "localhost is not in /etc/hosts at all",
			getent:   `exit 2`,
			wantCode: 1,
			wantOut:  "/etc/hosts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubs := append([]stub{{name: "getent", body: tt.getent}}, passthrough(t, "grep")...)

			out, code := runScript(t, script, stubs...)

			assert.Equal(t, tt.wantCode, code, "output: %s", out)
			if tt.wantOut != "" {
				assert.Contains(t, out, tt.wantOut)
			}
		})
	}
}

// TestCheckDeckhouseUserScript: a leftover deckhouse account from an earlier cluster collides with
// the one bashible creates, and the failure surfaces much later as files owned by the wrong uid.
func TestCheckDeckhouseUserScript(t *testing.T) {
	script := renderPreflightScript(t, "check_deckhouse_user.sh")

	tests := []struct {
		name     string
		id       string // stubbed `id`
		getent   string // stubbed `getent`
		sudo     string // stubbed `sudo`
		wantCode int
		wantOut  string
	}{
		{
			name:     "the node has no deckhouse account",
			id:       `exit 1`,
			getent:   `exit 2`,
			sudo:     `exit 1`,
			wantCode: 0,
		},
		{
			name:     "the account is the one Deckhouse creates",
			id:       `echo 64535`,
			getent:   `echo "deckhouse:x:64535:"`,
			sudo:     `exit 1`,
			wantCode: 0,
		},
		{
			// A user of that name from something else entirely, with a different id.
			name:     "an unrelated account holds the name",
			id:       `echo 1001`,
			getent:   `echo "deckhouse:x:1001:"`,
			wantCode: 1,
			wantOut:  "uid=1001, gid=1001 (expected 64535)",
		},
		{
			// Half-cleaned: the user was removed and the group left behind.
			name:     "the group is left over on its own",
			id:       `exit 1`,
			getent:   `echo "deckhouse:x:64535:"`,
			wantCode: 1,
			wantOut:  "unexpected id",
		},
		{
			name:     "the account has sudo rights",
			id:       `echo 64535`,
			getent:   `echo "deckhouse:x:64535:"`,
			sudo:     `echo "(ALL) NOPASSWD: ALL"`,
			wantCode: 1,
			wantOut:  "security risk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sudo := tt.sudo
			if sudo == "" {
				sudo = `exit 1`
			}

			stubs := append([]stub{
				{name: "id", body: tt.id},
				{name: "getent", body: tt.getent},
				{name: "sudo", body: sudo},
			}, passthrough(t, "cut", "grep")...)

			out, code := runScript(t, script, stubs...)

			assert.Equal(t, tt.wantCode, code, "output: %s", out)
			if tt.wantOut != "" {
				assert.Contains(t, out, tt.wantOut)
			}
			if tt.wantCode != 0 {
				// Every failure has to carry the way out; this is the check an operator
				// most often meets on a node they are reusing.
				assert.Contains(t, out, "cleanup_static_node.sh")
			}
		})
	}
}

// TestCheckPortsScriptWithoutPython: the script needs a python to open a socket with, and says so
// when there is none.
func TestCheckPortsScriptWithoutPython(t *testing.T) {
	script := renderPreflightScript(t, "check_ports.sh")

	// PATH holds cat and nothing resembling a python, which is what an image stripped down to
	// the minimum looks like.
	stubs := append([]stub{
		{name: "ps", body: `exit 1`},
		{name: "pkill", body: `exit 0`},
		{name: "sleep", body: `exit 0`},
	}, passthrough(t, "cat")...)

	out, code := runScript(t, script, stubs...)

	assert.Contains(t, out, "Python not found")
	assert.NotEqual(t, 0, code)

	// The script used to carry on without a python and report every port as closed, sending the
	// reader to the firewall — and then print SUCCESS underneath the failure.
	assert.NotContains(t, out, "Port 6443 is closed")
	assert.NotContains(t, out, "SUCCESS")
}

// fakePython stands in for the python the ports script opens sockets with. It tells the two
// snippets apart by their source: one starts a server, the other tries to connect. Ports named in
// $BUSY answer before any server is started, which is the "something else already holds it" case.
const fakePython = `
src=$(cat)

case "$src" in
  *HTTPServer*)
    rest=${src#*HTTPServer((\"\", }
    port=${rest%%)*}
    touch "$STATE/$port"
    while :; do sleep 1; done
    ;;
  *)
    rest=${src#*127.0.0.1:}
    port=${rest%%\'*}

    case " $BUSY " in
      *" $port "*) exit 0 ;;
    esac

    # check_port probes twice: once before it starts a server, to see whether the port is
    # already held, and once after. The first can answer at once — nothing can be listening
    # yet. Only the second waits, and then only until the server stub gets going; a shell
    # standing in for python does not always beat the script's 100ms.
    if [ ! -f "$STATE/probed-$port" ]; then
      touch "$STATE/probed-$port"
      exit 1
    fi

    i=0
    while [ $i -lt 40 ]; do
      if [ -f "$STATE/$port" ]; then exit 0; fi
      sleep 0.05
      i=$((i+1))
    done
    exit 1
    ;;
esac
`

// TestCheckPortsScript covers the control flow around the port probes. It was restructured because
// the script printed the failure for a port and then SUCCESS on the line below it, which reads as
// the check having passed.
func TestCheckPortsScript(t *testing.T) {
	tests := []struct {
		name string
		// busy lists the ports something else already holds.
		busy        []string
		wantCode    int
		wantMissing []string
		wantPresent []string
		// wantSuccesses is how many groups may report SUCCESS. The script used to print the
		// failure for a port and SUCCESS underneath it, so counting them is the assertion.
		wantSuccesses int
	}{
		{
			name:          "every port is free",
			wantCode:      0,
			wantMissing:   []string{"is not available"},
			wantSuccesses: 5,
		},
		{
			name:     "the API port is taken",
			busy:     []string{"6443"},
			wantCode: 1,
			// The other groups are still fine and still say so. What must not appear is a
			// SUCCESS under the group that just failed.
			wantPresent:   []string{"Port 6443 is not available but is required by the Kubernetes API server"},
			wantSuccesses: 4,
		},
		{
			name:          "one etcd port is taken",
			busy:          []string{"2380"},
			wantCode:      1,
			wantPresent:   []string{"Port 2380 is not available"},
			wantSuccesses: 4,
		},
		{
			// The port kubernetes-api-proxy needs. A previous install still running on the
			// node holds it, and the bootstrap otherwise reaches "kubernetes-api-proxy not
			// running after 200s" with no idea why.
			name:          "the api-proxy port is held by something else",
			busy:          []string{"6445"},
			wantCode:      1,
			wantPresent:   []string{"required by kubernetes-api-proxy", "ss -lntp | grep :6445"},
			wantSuccesses: 4,
		},
		{
			name:          "kubelet's port is taken",
			busy:          []string{"10250"},
			wantCode:      1,
			wantPresent:   []string{"required by kubelet"},
			wantSuccesses: 4,
		},
		{
			// dhctl brings the registry packages proxy up on this port itself, and a leftover
			// from an earlier run surfaces as "Cannot bring up registry packages proxy tunnel".
			name:          "the registry proxy ports are taken",
			busy:          []string{"5001", "5444"},
			wantCode:      1,
			wantPresent:   []string{"the in-cluster registry", "registry packages proxy"},
			wantSuccesses: 4,
		},
		{
			name:          "nothing is free",
			busy:          []string{"6443", "2379", "2380", "10250", "6445", "6480", "5001", "5444"},
			wantCode:      1,
			wantPresent:   []string{"Port 6443", "Port 2379", "Port 10250", "Port 6480"},
			wantSuccesses: 0,
		},
	}

	script := renderPreflightScript(t, "check_ports.sh")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := t.TempDir()
			stubs := append([]stub{
				{name: "python3", body: fakePython},
			}, passthrough(t, "cat", "sed", "head", "printf", "touch", "sleep", "ps", "pkill")...)

			out, code := runScriptWithEnv(t, script,
				[]string{"STATE=" + state, "BUSY=" + strings.Join(tt.busy, " ")}, stubs...)

			assert.Equal(t, tt.wantCode, code, "output:\n%s", out)
			for _, want := range tt.wantPresent {
				assert.Contains(t, out, want)
			}
			for _, unwanted := range tt.wantMissing {
				assert.NotContains(t, out, unwanted)
			}
			assert.Equal(t, tt.wantSuccesses, strings.Count(out, "SUCCESS"),
				"a group that failed must not also report SUCCESS; output:\n%s", out)
		})
	}
}
