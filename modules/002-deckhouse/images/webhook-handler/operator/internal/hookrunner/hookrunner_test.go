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

package hookrunner

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	serverScript = "../../../src/python_hook_server.py"
	// python_lib holds the sitecustomize.py the image puts on PYTHONPATH.
	pythonLib = "../../../../../../../python_lib"
)

// startServer starts the real python_hook_server.py and waits until it accepts connections.
func startServer(t *testing.T) *Runner {
	t.Helper()

	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}

	lib, err := filepath.Abs(pythonLib)
	require.NoError(t, err)
	t.Setenv("PYTHONPATH", lib)

	// A short path: unix socket paths are limited to about a hundred bytes, and TMPDIR may be long.
	dir, err := os.MkdirTemp("/tmp", "phs")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	r := New("python3", serverScript, filepath.Join(dir, "s.sock"), log.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Serve(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	require.Eventually(t, func() bool {
		conn, err := net.Dial("unix", r.socketPath)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 15*time.Second, 20*time.Millisecond, "python hook server did not start")

	return r
}

// writeHook writes an executable hook under <root>/<module>/webhooks/validating, the layout
// webhook-handler copies module webhooks into.
func writeHook(t *testing.T, root, name, body string) string {
	t.Helper()

	path := filepath.Join(root, "002-deckhouse", "webhooks", "validating", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))

	return path
}

func run(t *testing.T, r *Runner, path string, env ...string) (string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(path, "--flag")
	cmd.Dir = filepath.Dir(path)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_, err := r.Run(context.Background(), cmd)

	return stdout.String(), stderr.String(), err
}

func TestRunner_PythonHookRunsAsAScript(t *testing.T) {
	r := startServer(t)
	root := t.TempDir()

	// Sits next to the hook: `python3 hook.py` puts the hook directory first on sys.path.
	writeHook(t, root, "sibling.py", "#!/usr/bin/python3\nVALUE = 'sibling'\n")
	// The module lib path that sitecustomize.py adds for webhook-handler hooks.
	libDir := filepath.Join(root, "002-deckhouse", "webhooks", "lib", "python", "dist")
	require.NoError(t, os.MkdirAll(libDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "modlib.py"), []byte("VALUE = 'modlib'\n"), 0o644))

	hook := writeHook(t, root, "hook.py", `#!/usr/bin/python3
import os, sys
import sibling, modlib
print("name", __name__)
print("argv", " ".join(sys.argv[1:]))
print("cwd", os.getcwd() == os.path.dirname(os.path.realpath(__file__)))
print("env", os.environ["HOOK_INPUT"])
print("imports", sibling.VALUE, modlib.VALUE)
print("ppid", os.getppid())
print("to stderr", file=sys.stderr)
`)

	stdout, stderr, err := run(t, r, hook, "HOOK_INPUT=from-request")
	require.NoError(t, err, stderr)

	assert.Contains(t, stdout, "name __main__\n")
	assert.Contains(t, stdout, "argv --flag\n")
	assert.Contains(t, stdout, "cwd True\n")
	assert.Contains(t, stdout, "env from-request\n")
	assert.Contains(t, stdout, "imports sibling modlib\n")
	assert.Equal(t, "to stderr\n", stderr)
	// Forked by the server rather than started by this process.
	assert.NotContains(t, stdout, "ppid "+strconv.Itoa(os.Getpid())+"\n")
}

func TestRunner_Failures(t *testing.T) {
	r := startServer(t)
	root := t.TempDir()

	t.Run("exit code", func(t *testing.T) {
		hook := writeHook(t, root, "exit.py", "#!/usr/bin/python3\nimport sys\nprint('bad input', file=sys.stderr)\nsys.exit(3)\n")

		_, stderr, err := run(t, r, hook)
		require.ErrorContains(t, err, "exit status 3")
		assert.Equal(t, "bad input\n", stderr)
	})

	t.Run("exit with a message", func(t *testing.T) {
		hook := writeHook(t, root, "exitmsg.py", "#!/usr/bin/python3\nimport sys\nsys.exit('denied')\n")

		_, stderr, err := run(t, r, hook)
		require.ErrorContains(t, err, "exit status 1")
		assert.Equal(t, "denied\n", stderr)
	})

	t.Run("exit zero", func(t *testing.T) {
		hook := writeHook(t, root, "exit0.py", "#!/usr/bin/python3\nimport sys\nsys.exit(0)\n")

		_, _, err := run(t, r, hook)
		require.NoError(t, err)
	})

	t.Run("uncaught exception", func(t *testing.T) {
		hook := writeHook(t, root, "raise.py", "#!/usr/bin/python3\nraise ValueError('broken hook')\n")

		_, stderr, err := run(t, r, hook)
		require.ErrorContains(t, err, "exit status 1")
		assert.Contains(t, stderr, "Traceback")
		assert.Contains(t, stderr, "ValueError: broken hook")
	})
}

func TestRunner_RequestsDoNotShareState(t *testing.T) {
	r := startServer(t)
	root := t.TempDir()

	dirty := writeHook(t, root, "dirty.py", `#!/usr/bin/python3
import json, os
json.leftover = True
os.environ["LEFTOVER"] = "1"
`)
	check := writeHook(t, root, "check.py", `#!/usr/bin/python3
import json, os
print(hasattr(json, "leftover"), "LEFTOVER" in os.environ)
`)

	_, stderr, err := run(t, r, dirty)
	require.NoError(t, err, stderr)

	stdout, stderr, err := run(t, r, check)
	require.NoError(t, err, stderr)
	assert.Equal(t, "False False\n", stdout)
}

func TestRunner_HookCodeIsCachedPerContent(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv("TMPDIR", cacheRoot)
	r := startServer(t)
	root := t.TempDir()
	cacheDir := filepath.Join(cacheRoot, "python-hook-server-code")

	hook := writeHook(t, root, "cached.py", "#!/usr/bin/python3\nprint('one')\n")
	for range 2 {
		stdout, stderr, err := run(t, r, hook)
		require.NoError(t, err, stderr)
		assert.Equal(t, "one\n", stdout)
	}
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "two runs of the same file share one compiled entry")

	// Same size, new content: the cached code must not be reused.
	require.NoError(t, os.WriteFile(hook, []byte("#!/usr/bin/python3\nprint('two')\n"), 0o755))
	stdout, stderr, err := run(t, r, hook)
	require.NoError(t, err, stderr)
	assert.Equal(t, "two\n", stdout)

	entries, err = os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestRunner_HookCanRunSubprocesses(t *testing.T) {
	r := startServer(t)

	// system_resources.py shells out to kubectl; the server ignores SIGCHLD, its children must not.
	hook := writeHook(t, t.TempDir(), "sub.py", `#!/usr/bin/python3
import subprocess
proc = subprocess.run(["sh", "-c", "echo child; exit 4"], capture_output=True, text=True)
print(proc.returncode, proc.stdout.strip())
`)

	stdout, stderr, err := run(t, r, hook)
	require.NoError(t, err, stderr)
	assert.Equal(t, "4 child\n", stdout)
}

func TestRunner_StartsAProcess(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	root := t.TempDir()

	t.Run("for a hook in another language", func(t *testing.T) {
		r := startServer(t)
		hook := writeHook(t, root, "hook.sh", "#!/bin/sh\necho \"shell $PPID\"\n")

		stdout, _, err := run(t, r, hook)
		require.NoError(t, err)
		assert.Equal(t, "shell "+strconv.Itoa(os.Getpid())+"\n", stdout)
	})

	t.Run("for a python hook while the server is down", func(t *testing.T) {
		r := New("python3", serverScript, filepath.Join(root, "missing.sock"), log.NewNop())
		hook := writeHook(t, root, "down.py", "#!/usr/bin/python3\nimport os\nprint('ppid', os.getppid())\n")

		stdout, stderr, err := run(t, r, hook)
		require.NoError(t, err, stderr)
		assert.Equal(t, "ppid "+strconv.Itoa(os.Getpid())+"\n", stdout)
	})
}

func TestIsPythonScript(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]bool{
		"#!/usr/bin/python3\nprint(1)\n":         true,
		"#!/usr/bin/env python3\n":               true,
		"#!/usr/bin/env bash\n# python\n":        false,
		"print('no shebang')\n":                  false,
		"":                                       false,
		"#!/bin/sh\n" + strings.Repeat("x", 300): false,
	}

	i := 0
	for body, want := range cases {
		path := filepath.Join(dir, strconv.Itoa(i))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
		assert.Equal(t, want, isPythonScript(path), "%q", body)
		i++
	}

	assert.False(t, isPythonScript(filepath.Join(dir, "missing")))
}
