// Copyright 2021 Flant JSC
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

package tomb

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testReadyEnv    = "READY_MSG"
	testWorkEnv     = "WORK_MSG"
	testShutdownEnv = "SHUTDOWN_MSG"
	testExitCodeEnv = "EXIT_CODE"

	testShouldHandleInterruptEnv = "HANDLE_INTERRUPT"
)

type testRunResult struct {
	code int
	out  []string
	err  error
}

type testActionParams struct {
	ready, work, shutdown string
	handleInterrupt       string
	afterReady            func(t *testing.T, cmd *exec.Cmd, stdin io.Writer)
	exitCode              int
}

// readyDetector buffers all child output and closes readyCh the first time
// `target` appears as a complete line. The parent uses it to synchronize with
// the subprocess without any time-based waiting: it blocks on Ready() until
// the child has reached its announce point, then drives the rest of the test
// over signals or stdin.
type readyDetector struct {
	target  string
	readyCh chan struct{}

	mu   sync.Mutex
	buf  bytes.Buffer
	line []byte
	seen bool
}

func newReadyDetector(target string) *readyDetector {
	return &readyDetector{target: target, readyCh: make(chan struct{})}
}

func (d *readyDetector) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.buf.Write(p)
	for _, b := range p {
		if b != '\n' {
			d.line = append(d.line, b)
			continue
		}
		if !d.seen && string(d.line) == d.target {
			d.seen = true
			close(d.readyCh)
		}
		d.line = d.line[:0]
	}
	return len(p), nil
}

func (d *readyDetector) Ready() <-chan struct{} { return d.readyCh }

func (d *readyDetector) Output() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.buf.String()
}

func runAction(t *testing.T, p *testActionParams) *testRunResult {
	//nolint:gosec
	cmd := exec.Command(os.Args[0], "-test.run=TestAction")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=%s", testReadyEnv, p.ready),
		fmt.Sprintf("%s=%s", testWorkEnv, p.work),
		fmt.Sprintf("%s=%s", testShutdownEnv, p.shutdown),
		fmt.Sprintf("%s=%v", testExitCodeEnv, p.exitCode),
		fmt.Sprintf("%s=%v", testShouldHandleInterruptEnv, p.handleInterrupt),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return &testRunResult{err: fmt.Errorf("stdin pipe: %w", err)}
	}

	detector := newReadyDetector(p.ready)
	cmd.Stdout = detector
	cmd.Stderr = detector

	if err := cmd.Start(); err != nil {
		return &testRunResult{err: fmt.Errorf("cannot start test: %w", err)}
	}

	select {
	case <-detector.Ready():
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return &testRunResult{err: fmt.Errorf("timeout waiting for %q sentinel; got: %q", p.ready, detector.Output())}
	}

	if p.afterReady != nil {
		p.afterReady(t, cmd, stdin)
	}
	_ = stdin.Close()

	err = cmd.Wait()

	exitCode := 0
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode = exitError.ExitCode()
	}

	var lines []string
	for l := range strings.SplitSeq(detector.Output(), "\n") {
		if l == "" || l == "PASS" {
			continue
		}
		lines = append(lines, l)
	}

	return &testRunResult{out: lines, code: exitCode}
}

func assertRunResult(t *testing.T, res, expected *testRunResult) {
	require.NoError(t, res.err)
	require.Equal(t, expected.code, res.code, fmt.Sprintf("incorrect exit code. Need=%v, got=%v", expected.code, res.code))
	require.Equal(t, len(expected.out), len(res.out), fmt.Sprintf("incorrect outputs len. Need=%v, got=%v: %v", len(expected.out), len(res.out), res.out))

	for i, e := range expected.out {
		r := res.out[i]
		require.Equal(t, e, r, fmt.Sprintf("incorrect output line %v. Need=%v, got=%v", i, e, r))
	}
}

// continueAction unblocks the subprocess work goroutine, letting it print
// "work" and call Shutdown for the normal-exit case.
func continueAction(t *testing.T, _ *exec.Cmd, stdin io.Writer) {
	if _, err := io.WriteString(stdin, "continue\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
}

// sendSignalAction sends sig to the subprocess. For signals that tomb handles
// (SIGINT/SIGTERM/SIGUSR1/SIGUSR2), the work goroutine never gets past its
// stdin read, so we deliberately leave stdin empty.
func sendSignalAction(sig os.Signal) func(t *testing.T, cmd *exec.Cmd, stdin io.Writer) {
	return func(t *testing.T, cmd *exec.Cmd, _ io.Writer) {
		if err := cmd.Process.Signal(sig); err != nil {
			t.Fatalf("cannot send signal %v to process %v: %v", sig, cmd.Process.Pid, err)
		}
	}
}

// sendIgnoredSignalAction sends a signal that tomb does not interpret (e.g.
// SIGALRM). The subprocess is not interrupted, so we still feed "continue" so
// it can complete the normal flow.
func sendIgnoredSignalAction(sig os.Signal) func(t *testing.T, cmd *exec.Cmd, stdin io.Writer) {
	return func(t *testing.T, cmd *exec.Cmd, stdin io.Writer) {
		if err := cmd.Process.Signal(sig); err != nil {
			t.Fatalf("cannot send signal %v to process %v: %v", sig, cmd.Process.Pid, err)
		}
		continueAction(t, cmd, stdin)
	}
}

// TestAction do not run directly!
func TestAction(t *testing.T) {
	ready := os.Getenv(testReadyEnv)
	work := os.Getenv(testWorkEnv)
	shutdown := os.Getenv(testShutdownEnv)

	if ready == "" || work == "" || shutdown == "" {
		t.Skip("Envs not set probably you can run test directly")
		return
	}

	exitWithCodeStr := os.Getenv(testExitCodeEnv)
	exitWithCode, err := strconv.Atoi(exitWithCodeStr)
	if err != nil {
		t.Fatalf("incorrect exit code %s: %v", exitWithCodeStr, err)
	}

	msg := &beforeInterruptMsg{}

	notifyReady := make(chan struct{})
	signalNotifyHook = func() { close(notifyReady) }

	go WaitForProcessInterruption(BeforeInterrupted{
		func(_ os.Signal) {
			msg.Interrupt()
		},
	})

	// Wait for signal.Notify to be registered. Without this, a signal racing
	// in between `go ...` and the goroutine's signal.Notify would either kill
	// the subprocess (SIGTERM/SIGINT default action) or be silently dropped
	// by the Go runtime (SIGUSR1 is _SigNotify-only — runtime ignores it
	// when no channel is registered).
	<-notifyReady

	RegisterOnShutdown("Test shutdown", func() {
		fmt.Printf("%s\n", shutdown)
	})

	// Announce readiness. The parent waits for this exact line on stdout
	// before deciding to send a signal or feed stdin — there is no time-based
	// coordination anywhere in this test.
	fmt.Printf("%s\n", ready)

	// The work goroutine only proceeds when the parent explicitly writes
	// "continue\n" to our stdin. In signal scenarios the parent never writes,
	// so this goroutine stays parked on Scan() and never races os.Exit by
	// printing "work" after Shutdown has already started.
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() && scanner.Text() == "continue" {
			fmt.Printf("%s\n", work)
			Shutdown(exitWithCode)
		}
	}()

	exitCode := WaitShutdown()

	handleInterruptMsgExpected := os.Getenv(testShouldHandleInterruptEnv)
	require.Equal(t, handleInterruptMsgExpected, msg.Msg(), "before interruption msg will", handleInterruptMsgExpected)

	os.Exit(exitCode)
}

func TestTomb(t *testing.T) {
	ready := "ready"
	work := "work"
	shutdown := "shutdown"

	cases := []struct {
		caseName     string
		params       *testActionParams
		expectedCode int
		expectedOut  []string
	}{
		{
			caseName: "Normal running exit with zero code and running shutdown",
			params: &testActionParams{
				ready:           ready,
				work:            work,
				shutdown:        shutdown,
				afterReady:      continueAction,
				handleInterrupt: "", // normal exit does not interrupt
			},
			expectedCode: 0,
			expectedOut:  []string{ready, work, shutdown},
		},

		{
			caseName: "Send SIGTERM exit with zero code and running shutdown and run before interruption funcs",
			params: &testActionParams{
				ready:           ready,
				work:            work,
				shutdown:        shutdown,
				afterReady:      sendSignalAction(syscall.SIGTERM),
				handleInterrupt: beforeInterruptMsgStr, // need to handle
			},
			expectedCode: 0,
			expectedOut: []string{
				ready,
				shutdown,
				`Graceful shutdown by "terminated" signal ...`,
			},
		},

		{
			caseName: "Send SIGINT exit with zero code and running shutdown and run before interruption funcs",
			params: &testActionParams{
				ready:           ready,
				work:            work,
				shutdown:        shutdown,
				afterReady:      sendSignalAction(syscall.SIGINT),
				handleInterrupt: beforeInterruptMsgStr, // need to handle
			},
			expectedCode: 0,
			expectedOut: []string{
				ready,
				shutdown,
				`Graceful shutdown by "interrupt" signal ...`,
			},
		},

		{
			caseName: "Send SIGUSR1 exit with 1 code and running shutdown and run before interruption funcs",
			params: &testActionParams{
				ready:           ready,
				work:            work,
				shutdown:        shutdown,
				afterReady:      sendSignalAction(syscall.SIGUSR1),
				handleInterrupt: beforeInterruptMsgStr, // need to handle
			},
			expectedCode: 1,
			expectedOut: []string{
				ready,
				shutdown,
				`Graceful shutdown by "user defined signal 1" signal ...`,
			},
		},

		{
			caseName: "Send another sig (exclude USR1,USR2,TERM,INT) should skipped (code 0 with shutdown)",
			params: &testActionParams{
				ready:           ready,
				work:            work,
				shutdown:        shutdown,
				afterReady:      sendIgnoredSignalAction(syscall.SIGALRM),
				handleInterrupt: "", // does not handle because we handle only USR1,USR2,TERM,INT in waitShutdown
			},
			expectedCode: 0,
			expectedOut:  []string{ready, work, shutdown},
		},
	}

	for _, c := range cases {
		t.Run(c.caseName, func(t *testing.T) {
			res := runAction(t, c.params)
			assertRunResult(t, res, &testRunResult{
				code: c.expectedCode,
				out:  c.expectedOut,
			})
		})
	}
}

const beforeInterruptMsgStr = "Handle interrupt"

type beforeInterruptMsg struct {
	m   sync.Mutex
	msg string
}

func (b *beforeInterruptMsg) Interrupt() {
	b.m.Lock()
	defer b.m.Unlock()

	b.msg = beforeInterruptMsgStr
}

func (b *beforeInterruptMsg) Msg() string {
	b.m.Lock()
	defer b.m.Unlock()

	return b.msg
}
