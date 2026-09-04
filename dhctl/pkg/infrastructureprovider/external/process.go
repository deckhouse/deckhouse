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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
// one context: cancelling it signals the process.
type ValidatorProcess struct {
	cmd *exec.Cmd
	wg  sync.WaitGroup

	cancel   context.CancelFunc
	stopOnce sync.Once
	stopErr  error
}

// StartValidatorProcess starts a validator process on the given endpoint and returns
// once it accepts connections, so a caller gets a process it can talk to right away.
// The caller is responsible for managing the endpoint lifecycle.
func StartValidatorProcess(ctx context.Context, binaryPath string, endpoint Endpoint) (*ValidatorProcess, error) {
	ctx, cancel := context.WithCancel(ctx)
	logger := dhlog.FromContext(ctx)

	ret := &ValidatorProcess{
		cancel: cancel,
	}

	withStop := func(err error) error {
		if stopErr := ret.Stop(); stopErr != nil {
			return errors.Join(err, stopErr)
		}
		return err
	}

	cmd := exec.CommandContext(
		ctx,
		binaryPath,
		server.ServeArgs(endpoint.Network(), endpoint.Address())...,
	)

	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = shutdownTimeout
	ret.cmd = cmd

	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, withStop(fmt.Errorf("stdout pipe: %w", err))
	}

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, withStop(fmt.Errorf("stderr pipe: %w", err))
	}

	logger.DebugContext(ctx, fmt.Sprintf("start validator: %s", cmd.String()))
	if err := cmd.Start(); err != nil {
		return nil, withStop(err)
	}

	ret.wg = sync.WaitGroup{}
	logWriter := func(line string) {
		logger.DebugContext(ctx, fmt.Sprintf("validator output: %s", line))
	}
	ret.wg.Go(func() {
		outputHandler(stdoutReader, logWriter)
	})
	ret.wg.Go(func() {
		outputHandler(stderrReader, logWriter)
	})

	if err := ret.waitReady(ctx, endpoint); err != nil {
		return nil, withStop(err)
	}

	if err := ctx.Err(); err != nil {
		return nil, withStop(err)
	}

	return ret, nil
}

// Stop gracefully stops the validator process. It is safe to call multiple times and
// reports the same result every time.
//
// The order matters: cmd.Wait is what closes the output pipes and what escalates
// SIGTERM to a kill after WaitDelay, so neither a validator that ignores the signal
// nor a child holding its pipes can keep Stop waiting.
func (v *ValidatorProcess) Stop() error {
	v.stopOnce.Do(func() {
		if v.cancel != nil {
			v.cancel()
			v.cancel = nil
		}

		if v.cmd != nil && v.cmd.Process != nil {
			if err := v.cmd.Wait(); !isOrdinaryStop(err) {
				v.stopErr = fmt.Errorf("stop validator process: %w", err)
			}
		}

		v.wg.Wait()
		v.cmd = nil
	})

	return v.stopErr
}

// waitReady blocks until the endpoint accepts a connection: the protocol has no
// readiness service. ctx is the process's own, so this returns the moment the process
// is gone — there is nothing left to wait for — and readyTimeout bounds the rest.
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
			if ctx.Err() != nil {
				return errors.New("exited before listening")
			}
			return fmt.Errorf("did not listen on %s within %s", endpoint, readyTimeout)
		case <-ticker.C:
		}
	}
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

func outputHandler(reader io.Reader, writer func(line string)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, maxLineSize), maxLineSize)

	for scanner.Scan() {
		writer(scanner.Text())
	}

	// os.ErrClosed is the normal end of a read: Stop's cmd.Wait closes the pipes
	// out from under these goroutines on purpose.
	err := scanner.Err()
	if err != nil &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, os.ErrClosed) {
		writer(fmt.Sprintf("scanner error: %v", err))
	}
}
