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

package external

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	shutdownTimeout = 2 * time.Second

	// readyProbe is how often the endpoint is probed while the validator binds it.
	// gRPC's own reconnect backoff starts at a second, which would be most of the
	// wall clock of a call that otherwise takes milliseconds.
	readyProbe = 10 * time.Millisecond

	// readyTimeout caps the wait for the validator to listen, so a binary that
	// starts but never listens does not eat the budget of the call that follows.
	readyTimeout = 10 * time.Second

	// maxLineSize bounds a single line of the validator's output.
	maxLineSize = 4 * 1024 * 1024
)

// ValidatorProcess manages a running validator process. Its whole lifetime hangs on
// one context: cancelling it signals the process, and the process going down cancels
// it back, so whatever waits on the validator is released either way.
type ValidatorProcess struct {
	cmd    *exec.Cmd
	cancel context.CancelCauseFunc

	stdout *output
	stderr *output

	waitWG  sync.WaitGroup
	waitErr error

	stopOnce sync.Once
	stopErr  error
}

// StartValidatorProcess starts a validator process on the given endpoint and returns
// once it accepts connections, so a caller gets a process it can talk to right away.
// The caller is responsible for managing the endpoint lifecycle.
func StartValidatorProcess(ctx context.Context, binaryPath string, endpoint Endpoint) (*ValidatorProcess, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	logger := dhlog.FromContext(ctx)

	stdout := newOutput(func(line string) {
		logger.DebugContext(ctx, fmt.Sprintf("validator: %s", line))
	})
	stderr := newOutput(func(line string) {
		logger.DebugContext(ctx, fmt.Sprintf("validator: %s", line))
	})

	ret := &ValidatorProcess{
		cancel: cancel,
		stdout: stdout,
		stderr: stderr,
		cmd:    validatorCmd(ctx, binaryPath, endpoint, stdout, stderr),
	}

	withStop := func(err error) error {
		if stopErr := ret.Stop(); stopErr != nil {
			return errors.Join(err, stopErr)
		}
		return err
	}

	logger.DebugContext(ctx, fmt.Sprintf("start validator: %s", ret.cmd.String()))

	if err := ret.cmd.Start(); err != nil {
		return nil, withStop(err)
	}

	ret.waitWG.Go(func() {
		ret.waitErr = ret.cmd.Wait()
		_ = syscall.Kill(-ret.cmd.Process.Pid, syscall.SIGKILL)
		ret.cancel(fmt.Errorf("validator exited: %w", ret.waitErr))
	})

	if err := ret.waitReady(ctx, endpoint); err != nil {
		return nil, withStop(err)
	}
	return ret, nil
}

// Stop gracefully stops the validator process. It is safe to call multiple times and
// reports the same result every time.
//
// The wait is bounded by cmd.WaitDelay: cancelling starts its timer, and when it
// expires os/exec kills the child and closes its pipes, so neither a validator that
// ignores SIGTERM nor a child holding the pipes can keep Stop waiting.
func (v *ValidatorProcess) Stop() error {
	v.stopOnce.Do(func() {
		v.cancel(nil)
		v.waitWG.Wait()

		v.stdout.Flush()
		v.stderr.Flush()

		if !isOrdinaryStop(v.waitErr) {
			v.stopErr = fmt.Errorf("stop validator process: %w", v.waitErr)
		}
	})
	return v.stopErr
}

// waitReady blocks until the endpoint accepts a connection: the protocol has no
// readiness service. The context is cancelled once the process is gone, so a
// validator that dies is reported as dead instead of costing the whole readyTimeout.
func (v *ValidatorProcess) waitReady(ctx context.Context, endpoint Endpoint) error {
	deadline, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()

	ticker := time.NewTicker(readyProbe)
	defer ticker.Stop()

	for {
		if endpoint.Accepting(readyProbe) {
			return nil
		}

		select {
		case <-deadline.Done():
			// The deadline is derived from the process context, so it covers both
			// ways of giving up. Only the cause tells them apart, and it is nil
			// while the process itself is still alive.
			if cause := context.Cause(ctx); cause != nil {
				return fmt.Errorf("%w, never listened on %s", cause, endpoint)
			}

			return fmt.Errorf("did not listen on %s within %s", endpoint, readyTimeout)
		case <-ticker.C:
		}
	}
}

// validatorCmd is how dhctl runs a validator: in a process group of its own, so one
// that spawns helpers takes them down with it, and against writers rather than pipes,
// so os/exec owns the copying and cmd.Wait waits for it.
func validatorCmd(ctx context.Context, binaryPath string, endpoint Endpoint, stdout, stderr io.Writer) *exec.Cmd {
	cmd := exec.CommandContext(
		ctx,
		binaryPath,
		server.ServeArgs(endpoint.Network(), endpoint.Address())...,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = shutdownTimeout
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

// isOrdinaryStop reports whether err is one of the ways a validator we asked to stop
// is supposed to go down: a non-zero exit (SIGTERM is a non-zero exit), the
// cancellation we sent, or pipes closed once WaitDelay ran out. What is left is a
// process the runtime could not signal or reap — the only thing a caller can act on.
func isOrdinaryStop(err error) bool {
	if err == nil {
		return true
	}

	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) ||
		errors.Is(err, exec.ErrWaitDelay) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, os.ErrProcessDone)
}

// output is what a validator writes: every complete line reaches the log the moment it
// arrives. os/exec's copying goroutine may outlive Wait when WaitDelay expires, so the
// state is locked.
type output struct {
	log func(line string)

	mu      sync.Mutex
	pending []byte
}

func newOutput(log func(line string)) *output {
	return &output{log: log}
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.pending = append(o.pending, p...)
	for {
		end := bytes.IndexByte(o.pending, '\n')
		if end < 0 {
			// A process that never writes a newline must not grow the buffer forever.
			if len(o.pending) >= maxLineSize {
				o.log(string(o.pending))
				o.pending = o.pending[:0]
			}

			return len(p), nil
		}

		o.log(strings.TrimSuffix(string(o.pending[:end+1]), "\n"))
		o.pending = o.pending[end+1:]
	}
}

// Flush logs a last line the process left unterminated.
func (o *output) Flush() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.pending) > 0 {
		o.log(string(o.pending))
		o.pending = o.pending[:0]
	}
}
