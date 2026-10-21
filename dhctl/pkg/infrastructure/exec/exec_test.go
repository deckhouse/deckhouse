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

package exec

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	// helperInterruptsToExitEnv sets how many SIGINTs the helper process takes to exit.
	helperInterruptsToExitEnv = "DHCTL_EXEC_TEST_HELPER_INTERRUPTS_TO_EXIT"
	// helperExitImmediately makes the helper exit on its own, without any signal.
	helperExitImmediately = 0
	// helperNeverExit makes the helper handle SIGINT without ever exiting on it.
	helperNeverExit = -1

	helperReadyLine = "ready"
	// helperInterruptedExitCode is what the helper exits with on its last SIGINT.
	helperInterruptedExitCode = 1
	// killedExitCode is what ProcessState reports for a process killed by a signal.
	killedExitCode = -1

	// testGracePeriod keeps the two SIGINTs as far apart as the code does when it
	// skips the grace period: closer ones may merge into one.
	testGracePeriod = forcedStopSignalGap
	testKillDelay   = 100 * time.Millisecond
	// testNotStoppedWindow is how long a utility that must keep running is watched:
	// longer than the signal gap, so that a grace period cut down to it is noticed.
	testNotStoppedWindow = 3 * forcedStopSignalGap

	testWaitTimeout = 10 * time.Second
	testWaitTick    = 10 * time.Millisecond

	// helperLifetime ends a helper left behind by a test that failed before killing it.
	helperLifetime = 3 * testWaitTimeout
)

func TestExecForcedStop(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		interruptsToExit int
		gracePeriod      time.Duration
		signalGap        time.Duration // forcedStopSignalGap by default
		abandon          bool
		wantForced       bool
		wantExitCode     int
	}{
		"utility exiting on the first SIGINT is not forced": {
			interruptsToExit: 1,
			// Exec must be done before the grace period ends, even on a loaded machine.
			gracePeriod:  testWaitTimeout,
			wantForced:   false,
			wantExitCode: helperInterruptedExitCode,
		},
		"second SIGINT stops a utility finishing its work": {
			interruptsToExit: 2,
			gracePeriod:      testGracePeriod,
			wantForced:       true,
			wantExitCode:     helperInterruptedExitCode,
		},
		"SIGKILL stops a utility ignoring SIGINT": {
			interruptsToExit: helperNeverExit,
			gracePeriod:      testGracePeriod,
			wantForced:       true,
			wantExitCode:     killedExitCode,
		},
		// Abandoned is canceled together with the operation, as the stream context is
		// in server mode: the second SIGINT must not merge into the first one.
		"abandoned utility gets the second SIGINT without the grace period": {
			interruptsToExit: 2,
			// Far beyond testWaitTimeout: only skipping it lets Exec return in time.
			gracePeriod:  time.Hour,
			abandon:      true,
			wantForced:   true,
			wantExitCode: helperInterruptedExitCode,
		},
		"abandoned utility ignoring SIGINT is killed": {
			interruptsToExit: helperNeverExit,
			gracePeriod:      time.Hour,
			abandon:          true,
			wantForced:       true,
			wantExitCode:     killedExitCode,
		},
		"abandoned utility exiting within the signal gap is not forced": {
			interruptsToExit: 1,
			gracePeriod:      time.Hour,
			// Far beyond testWaitTimeout: the utility surely exits within it.
			signalGap:    time.Hour,
			abandon:      true,
			wantForced:   false,
			wantExitCode: helperInterruptedExitCode,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stream, abandon := context.WithCancel(t.Context())
			defer abandon()

			var abandoned <-chan struct{}
			if tt.abandon {
				abandoned = stream.Done()
			}

			ctx, cancel := context.WithCancel(withForcedStop(stream, forcedStop{
				gracePeriod: tt.gracePeriod,
				signalGap:   cmp.Or(tt.signalGap, forcedStopSignalGap),
				killDelay:   testKillDelay,
				abandoned:   abandoned,
			}))
			defer cancel()

			h := startHelper(ctx, t, tt.interruptsToExit)
			if tt.abandon {
				abandon()
			} else {
				cancel()
			}

			res := h.wait(t)

			require.Error(t, res.err)
			require.Equal(t, tt.wantExitCode, res.exitCode)
			if tt.wantForced {
				require.ErrorContains(t, res.err, "force stopped")
			} else {
				require.NotContains(t, res.err.Error(), "force stopped")
			}
		})
	}
}

func TestExecAbandonedDuringGracePeriod(t *testing.T) {
	t.Parallel()

	stream, abandon := context.WithCancel(t.Context())
	defer abandon()

	ctx, cancel := context.WithCancel(withForcedStop(stream, forcedStop{
		gracePeriod: time.Hour,
		signalGap:   forcedStopSignalGap,
		killDelay:   testKillDelay,
		abandoned:   stream.Done(),
	}))
	defer cancel()

	h := startHelper(ctx, t, 2)

	// A soft cancel: the utility keeps finishing its work in the grace period.
	cancel()
	require.Never(t, h.exited, testNotStoppedWindow, testWaitTick,
		"utility was stopped before the grace period ended")

	abandon()
	res := h.wait(t)

	require.ErrorContains(t, res.err, "force stopped")
	require.Equal(t, helperInterruptedExitCode, res.exitCode)
}

func TestExecWithoutForcedStopSendsSingleInterrupt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := startHelper(ctx, t, 2)
	cancel()

	require.Never(t, h.exited, testNotStoppedWindow, testWaitTick,
		"utility was stopped without forced stop enabled")

	// Clean up the helper that is expected to be still running.
	require.NoError(t, syscall.Kill(-h.pid, syscall.SIGKILL))
	h.wait(t)
}

func TestExecWithForcedStopDoesNotSignalUtilityWithoutCancellation(t *testing.T) {
	t.Parallel()

	ctx := withForcedStop(t.Context(), forcedStop{
		gracePeriod: testGracePeriod,
		signalGap:   forcedStopSignalGap,
		killDelay:   testKillDelay,
	})

	// Any signal ends the helper: a long run that is not canceled must get none.
	h := startHelper(ctx, t, 1)

	require.Never(t, h.exited, testNotStoppedWindow, testWaitTick,
		"utility was signaled without cancellation")

	// Clean up the helper that is expected to be still running.
	require.NoError(t, syscall.Kill(-h.pid, syscall.SIGKILL))
	h.wait(t)
}

func TestExecWithForcedStopKeepsSuccessfulExit(t *testing.T) {
	t.Parallel()

	ctx := withForcedStop(t.Context(), forcedStop{
		gracePeriod: testGracePeriod,
		signalGap:   forcedStopSignalGap,
		killDelay:   testKillDelay,
	})

	h := startHelper(ctx, t, helperExitImmediately)
	res := h.wait(t)

	require.NoError(t, res.err)
	require.Zero(t, res.exitCode)
}

// TestExecHelperProcess is not a real test: it is the utility that the tests
// above run, exiting after helperInterruptsToExitEnv SIGINTs.
func TestExecHelperProcess(t *testing.T) {
	value, ok := os.LookupEnv(helperInterruptsToExitEnv)
	if !ok {
		t.Skip("helper process for exec tests")
	}

	interruptsToExit, err := strconv.Atoi(value)
	require.NoError(t, err)

	time.AfterFunc(helperLifetime, func() { os.Exit(3) })

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, syscall.SIGINT)

	fmt.Printf("%s %d\n", helperReadyLine, os.Getpid())

	if interruptsToExit == helperExitImmediately {
		os.Exit(0)
	}

	received := 0
	for range interrupts {
		received++
		if received == interruptsToExit {
			os.Exit(helperInterruptedExitCode)
		}
	}
}

type execResult struct {
	exitCode int
	err      error
}

type helper struct {
	pid    int
	result <-chan execResult

	// Eventually and Never call exited from their own goroutines, and Never
	// returns without waiting for the last call.
	mu  sync.Mutex
	res *execResult
}

// startHelper runs the helper utility through Exec and returns once it is ready
// to handle signals. Cancellation sends SIGINT to its process group, as tofu and
// terraform commands do.
func startHelper(ctx context.Context, t *testing.T, interruptsToExit int) *helper {
	t.Helper()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExecHelperProcess$")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%d", helperInterruptsToExitEnv, interruptsToExit))
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}

	stdout := newReadyWriter()
	cmd.Stdout = stdout

	result := make(chan execResult, 1)
	go func() {
		exitCode, err := Exec(ctx, cmd, false)
		result <- execResult{exitCode: exitCode, err: err}
	}()

	select {
	case pid := <-stdout.ready:
		h := &helper{pid: pid, result: result}

		// The helper may ignore SIGINT: a failed test must not leave it running.
		t.Cleanup(func() {
			if !h.exited() {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		})

		return h
	case res := <-result:
		t.Fatalf("helper exited before it was ready: %v, output: %q", res.err, stdout.String())
	case <-time.After(testWaitTimeout):
		t.Fatalf("timeout waiting for helper to be ready, output: %q", stdout.String())
	}

	return nil
}

func (h *helper) wait(t *testing.T) execResult {
	t.Helper()

	require.Eventually(t, h.exited, testWaitTimeout, testWaitTick, "Exec did not return")

	h.mu.Lock()
	defer h.mu.Unlock()

	return *h.res
}

// exited reports whether Exec has returned, keeping its result.
func (h *helper) exited() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.res != nil {
		return true
	}

	select {
	case res := <-h.result:
		h.res = &res
		return true
	default:
		return false
	}
}

// readyWriter reports the helper pid once the helper prints its ready line.
type readyWriter struct {
	ready chan int

	mu   sync.Mutex
	buf  bytes.Buffer
	seen bool
}

func newReadyWriter() *readyWriter {
	return &readyWriter{ready: make(chan int, 1)}
}

func (w *readyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	if w.seen {
		return len(p), nil
	}

	for line := range strings.Lines(w.buf.String()) {
		// A line still being written may carry a partial pid.
		if !strings.HasSuffix(line, "\n") {
			break
		}

		pidStr, ok := strings.CutPrefix(strings.TrimSpace(line), helperReadyLine+" ")
		if !ok {
			continue
		}

		if pid, err := strconv.Atoi(pidStr); err == nil {
			w.seen = true
			w.ready <- pid
		}
	}

	return len(p), nil
}

func (w *readyWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}
