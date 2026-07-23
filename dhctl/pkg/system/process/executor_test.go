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

package process

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_Stop_WithStdoutPipeClose(t *testing.T) {
	cmd := exec.Command("yes", "out")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	ready := make(chan struct{})
	var once sync.Once

	e := NewDefaultExecutor(cmd)
	e.WithStdoutHandler(func(_ string) {
		once.Do(func() { close(ready) })
	})

	require.NoError(t, e.Start(), "start executor")

	waitForReady(t, ready, "stdout handler did not receive output")
	stopWithTimeout(t, e, cmd)
}

func TestExecutor_Stop_WithStderrPipeClose(t *testing.T) {
	cmd := exec.Command("sh", "-c", "yes err 1>&2")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	ready := make(chan struct{})
	var once sync.Once

	e := NewDefaultExecutor(cmd)
	e.WithStderrHandler(func(_ string) {
		once.Do(func() { close(ready) })
	})

	require.NoError(t, e.Start(), "start executor")

	waitForReady(t, ready, "stderr handler did not receive output")
	stopWithTimeout(t, e, cmd)
}

func TestExecutor_ReadFromStreams_StopsOnClosedReader(t *testing.T) {
	e := NewDefaultExecutor(exec.Command("echo", "test"))

	done := make(chan struct{})
	go func() {
		e.readFromStreams(&errReader{err: os.ErrClosed}, io.Discard)
		close(done)
	}()

	waitForReady(t, done, "readFromStreams must stop on closed reader")
}

func TestExecutor_ReadFromStreams_HandlesLastChunkOnEOF(t *testing.T) {
	e := NewDefaultExecutor(exec.Command("echo", "test"))
	e.CaptureStdout(nil)

	e.readFromStreams(&eofChunkReader{chunk: []byte("tail")}, io.Discard)

	assert.Equal(t, "tail", string(e.StdoutBytes()))
}

func TestExecutor_Stop_WithTimeoutRace_DoesNotPanic(t *testing.T) {
	for i := 0; i < 100; i++ {
		cmd := exec.Command("sleep", "1")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		e := NewDefaultExecutor(cmd)
		e.WithTimeout(time.Millisecond)

		require.NoError(t, e.Start(), "start executor")
		time.Sleep(time.Millisecond)

		done := make(chan struct{})
		go func() {
			e.Stop()
			close(done)
		}()

		waitForSignal(t, done, 5*time.Second, "executor stop timed out in timeout race", func() {
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		})
	}
}

func TestExecutor_Timeout_CleansUpStreamGoroutines(t *testing.T) {
	cmd := exec.Command("yes", "out")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	ready := make(chan struct{})
	var once sync.Once

	e := NewDefaultExecutor(cmd)
	e.WithTimeout(10 * time.Millisecond)
	e.WithStdoutHandler(func(_ string) {
		once.Do(func() { close(ready) })
	})
	t.Cleanup(e.forceClosePipes)

	require.NoError(t, e.Start(), "start executor")
	waitForReady(t, ready, "stdout handler did not receive output")
	waitForReady(t, e.waitCh, "executor did not stop on timeout")

	require.Eventually(t, func() bool {
		return countGoroutinesWithStack("process.(*Executor).readFromStreams") == 0 &&
			countGoroutinesWithStack("process.(*Executor).ConsumeLines") == 0
	}, 5*time.Second, 10*time.Millisecond, "executor timeout must clean up stream goroutines")
}

func TestExecutor_TimeoutGoroutine_StopsAfterProcessExit(t *testing.T) {
	e := NewDefaultExecutor(exec.Command("true"))
	e.WithTimeout(time.Hour)

	require.NoError(t, e.Start(), "start executor")
	waitForReady(t, e.waitCh, "executor did not finish")

	require.Eventually(t, func() bool {
		return countGoroutinesWithStack("process.(*Executor).waitForTimeout") == 0
	}, 5*time.Second, 10*time.Millisecond, "executor timeout goroutine must stop after process exit")
}

func TestExecutor_Stop_KillsProcessGroupChildren(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 60 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	e := NewDefaultExecutor(cmd)
	require.NoError(t, e.Start(), "start executor")

	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	})

	stopWithTimeout(t, e, cmd)

	require.Eventually(t, func() bool {
		return syscall.Kill(-pid, 0) != nil
	}, 5*time.Second, 10*time.Millisecond, "executor stop must kill process group children")
}

func TestExecutor_Run_CleansUpReadPipesOnNormalExit(t *testing.T) {
	e := NewDefaultExecutor(exec.Command("sh", "-c", "printf out; printf err >&2"))
	e.CaptureStdout(nil)
	e.CaptureStderr(nil)
	e.WithStdoutHandler(func(_ string) {})
	e.WithStderrHandler(func(_ string) {})
	t.Cleanup(e.forceClosePipes)

	require.NoError(t, e.Run(nil), "run executor")

	require.Eventually(t, func() bool {
		e.pipesMutex.Lock()
		defer e.pipesMutex.Unlock()

		return e.stdoutReadPipe == nil &&
			e.stderrReadPipe == nil &&
			e.stdoutHandlerReadPipe == nil &&
			e.stderrHandlerReadPipe == nil
	}, 5*time.Second, 10*time.Millisecond, "executor must clean up read-side pipes after normal exit")
}

func TestExecutor_Start_CleansUpPipesWhenCommandStartFails(t *testing.T) {
	e := NewDefaultExecutor(exec.Command("missing-binary-for-executor-test"))
	e.CaptureStdout(nil)
	e.CaptureStderr(nil)
	e.WithStdoutHandler(func(_ string) {})
	e.WithStderrHandler(func(_ string) {})

	err := e.Start()
	require.Error(t, err)

	require.Eventually(t, func() bool {
		e.pipesMutex.Lock()
		defer e.pipesMutex.Unlock()

		return e.stdoutPipeFile == nil &&
			e.stderrPipeFile == nil &&
			e.stdoutReadPipe == nil &&
			e.stderrReadPipe == nil &&
			e.stdoutHandlerReadPipe == nil &&
			e.stderrHandlerReadPipe == nil
	}, 5*time.Second, 10*time.Millisecond, "executor must clean up pipes when command start fails")
}

func waitForReady(t *testing.T, ready <-chan struct{}, timeoutMessage string) {
	t.Helper()
	waitForSignal(t, ready, 5*time.Second, timeoutMessage, nil)
}

func waitForSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, timeoutMessage string, onTimeout func()) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(timeout):
		if onTimeout != nil {
			onTimeout()
		}
		require.FailNow(t, timeoutMessage)
	}
}

func countGoroutinesWithStack(needle string) int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)

	return strings.Count(string(buf[:n]), needle)
}

func stopWithTimeout(t *testing.T, e *Executor, cmd *exec.Cmd) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		e.Stop()
		close(done)
	}()

	waitForSignal(t, done, 5*time.Second, "executor stop timed out", func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	})
}

type errReader struct {
	err error
}

func (r *errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

type eofChunkReader struct {
	chunk []byte
	sent  bool
}

func (r *eofChunkReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, io.EOF
	}
	r.sent = true
	n := copy(p, r.chunk)
	return n, io.EOF
}
