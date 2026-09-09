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

package distribution

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProc builds a process filesystem with the given command lines, keyed by
// process identifier.
func fakeProc(t *testing.T, processes map[int]string) string {
	t.Helper()

	root := t.TempDir()
	for pid, cmdline := range processes {
		dir := filepath.Join(root, strconv.Itoa(pid))
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644))
	}

	// Entries that are not process identifiers, which a real /proc is full of.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sys"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "uptime"), []byte("123"), 0o644))

	return root
}

func newRestarter(t *testing.T, processes map[int]string) (*SignalRestarter, *[]int) {
	t.Helper()

	var signalled []int
	restarter := &SignalRestarter{
		ProcessName: "registry",
		ProcDir:     fakeProc(t, processes),
		signal: func(pid int, _ syscall.Signal) error {
			signalled = append(signalled, pid)
			return nil
		},
	}
	return restarter, &signalled
}

func TestRestartSignalsTheRegistry(t *testing.T) {
	restarter, signalled := newRestarter(t, map[int]string{
		7:  "/registry\x00serve\x00/config/config.yaml\x00",
		11: "/pause\x00",
	})

	require.NoError(t, restarter.Restart())
	assert.Equal(t, []int{7}, *signalled)
}

// TestRestartIgnoresArguments guards against the syncer signalling itself: its own
// command line mentions the registry configuration path, which a naive substring
// match would happily match.
func TestRestartIgnoresArguments(t *testing.T) {
	restarter, signalled := newRestarter(t, map[int]string{
		7: "/registry-syncer\x00--config\x00/config/registry.yaml\x00",
		9: "/registry\x00serve\x00/config/config.yaml\x00",
	})

	require.NoError(t, restarter.Restart())
	assert.Equal(t, []int{9}, *signalled, "only the executable may be matched, never the arguments")
}

func TestRestartWithoutAMatchingProcess(t *testing.T) {
	restarter, signalled := newRestarter(t, map[int]string{
		11: "/pause\x00",
	})

	// Reported so the pass logs it, but the configuration is already on disk: the
	// registry reads it whenever it does come up.
	err := restarter.Restart()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no process named")
	assert.Empty(t, *signalled)
}

func TestRestartExcludesItself(t *testing.T) {
	restarter, signalled := newRestarter(t, map[int]string{
		os.Getpid(): "/registry\x00",
	})

	// A syncer that signals itself would restart-loop forever.
	err := restarter.Restart()
	require.Error(t, err)
	assert.Empty(t, *signalled)
}

func TestRestartSignalsEveryMatch(t *testing.T) {
	restarter, signalled := newRestarter(t, map[int]string{
		7: "/registry\x00serve\x00",
		8: "/registry\x00serve\x00",
	})

	require.NoError(t, restarter.Restart())
	assert.Len(t, *signalled, 2)
}

func TestRestartToleratesAProcessThatExited(t *testing.T) {
	root := fakeProc(t, map[int]string{7: "/registry\x00"})
	// A directory with no cmdline is what a process that exited mid-listing looks
	// like.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "99"), 0o755))

	var signalled []int
	restarter := &SignalRestarter{
		ProcessName: "registry",
		ProcDir:     root,
		signal: func(pid int, _ syscall.Signal) error {
			signalled = append(signalled, pid)
			return nil
		},
	}

	require.NoError(t, restarter.Restart())
	assert.Equal(t, []int{7}, signalled)
}

func TestRestartFailsOnAnUnreadableProcDir(t *testing.T) {
	restarter := &SignalRestarter{ProcessName: "registry", ProcDir: filepath.Join(t.TempDir(), "absent")}
	assert.Error(t, restarter.Restart())
}

func TestMatchesProcess(t *testing.T) {
	tests := []struct {
		name    string
		cmdline string
		want    string
		matches bool
	}{
		{name: "absolute path", cmdline: "/registry\x00serve\x00", want: "registry", matches: true},
		{name: "bare name", cmdline: "registry\x00", want: "registry", matches: true},
		{name: "nested path", cmdline: "/usr/bin/registry\x00", want: "registry", matches: true},
		{name: "no trailing NUL", cmdline: "/registry", want: "registry", matches: true},
		{name: "a different executable", cmdline: "/registry-syncer\x00", want: "registry", matches: false},
		{name: "the name only in an argument", cmdline: "/sh\x00-c\x00registry\x00", want: "registry", matches: false},
		{name: "empty command line", cmdline: "", want: "registry", matches: false},
		{name: "only NULs", cmdline: "\x00\x00", want: "registry", matches: false},
		{name: "no wanted name", cmdline: "/registry\x00", want: "", matches: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.matches, matchesProcess([]byte(tt.cmdline), tt.want))
		})
	}
}
