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
	announceTimeout = 10 * time.Second

	// Port 0: the kernel picks it, the validator binds it and announces what it got.
	loopbackAddress = "127.0.0.1:0"
)

type validatorOptions struct {
	binaryPath string
	endpoint   endpoint
	onLine     func(line string)
	wait       func(ctx context.Context) error
}

func (o validatorOptions) validate() error {
	if o.binaryPath == "" {
		return errors.New("binary path is required")
	}

	if err := o.endpoint.Validate(); err != nil {
		return fmt.Errorf("endpoint: %w", err)
	}
	return nil
}

// validatorProcess manages a running validator. Its whole lifetime hangs on one
// context: cancelling it signals the process, and the process going down cancels it
// back, so whatever waits on the validator is released either way.
type validatorProcess struct {
	cancel context.CancelCauseFunc
	cmd    *exec.Cmd

	stdout *outputHandler
	stderr *outputHandler

	waitWG  sync.WaitGroup
	waitErr error

	stopOnce sync.Once
	stopErr  error
}

// startValidatorProcess starts a validator, hands every line it writes to
// opt.onLine before logging it, and returns once opt.wait is satisfied.
func startValidatorProcess(ctx context.Context, opt validatorOptions) (*validatorProcess, error) {
	if err := opt.validate(); err != nil {
		return nil, fmt.Errorf("validator options: %w", err)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	logger := dhlog.FromContext(ctx)

	handleLine := func(line string) {
		if opt.onLine != nil {
			opt.onLine(line)
		}
		logger.DebugContext(ctx, fmt.Sprintf("validator: %s", line))
	}

	stdout := newOutputHandler(handleLine)
	stderr := newOutputHandler(handleLine)
	ret := &validatorProcess{
		cancel: cancel,
		stdout: stdout,
		stderr: stderr,
		cmd:    validatorCmd(ctx, opt.binaryPath, opt.endpoint, stdout, stderr),
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

	if opt.wait != nil {
		if err := opt.wait(ctx); err != nil {
			return nil, withStop(err)
		}
	}
	return ret, nil
}

// startListeningValidator starts a validator and waits for it to say where it listens,
// so a caller gets a process it can talk to right away.
func startListeningValidator(ctx context.Context, binaryPath string) (*validatorProcess, endpoint, error) {
	announcedCh := make(chan endpoint, 1)
	var ep endpoint

	catchEndpoint := func(line string) {
		network, address, ok := server.ParseListeningLine(line)
		if !ok {
			return
		}
		select {
		case announcedCh <- endpoint{Network: network, Address: address}:
		default:
		}
	}

	waitEndpoint := func(ctx context.Context) error {
		deadline, cancel := context.WithTimeout(ctx, announceTimeout)
		defer cancel()

		select {
		case announced := <-announcedCh:
			ep = announced
			return nil

		case <-deadline.Done():
			if cause := context.Cause(ctx); cause != nil {
				return fmt.Errorf("%w, never announced an endpoint", cause)
			}
			return fmt.Errorf("announced no endpoint within %s", announceTimeout)
		}
	}

	process, err := startValidatorProcess(ctx, validatorOptions{
		binaryPath: binaryPath,
		endpoint: endpoint{
			Address: loopbackAddress,
			Network: networkTCP,
		},
		onLine: catchEndpoint,
		wait:   waitEndpoint,
	})

	if err != nil {
		return nil, endpoint{}, err
	}
	return process, ep, nil
}

// Stop is safe to call multiple times and reports the same result every time. The wait
// is bounded by cmd.WaitDelay: cancelling starts its timer, and when it expires
// os/exec kills the child and closes its pipes.
func (v *validatorProcess) Stop() error {
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

// validatorCmd runs a validator in a process group of its own, so one that spawns
// helpers takes them down with it, and against writers rather than pipes, so os/exec
// owns the copying and cmd.Wait waits for it.
func validatorCmd(ctx context.Context, binaryPath string, ep endpoint, stdout, stderr io.Writer) *exec.Cmd {
	cmd := exec.CommandContext(
		ctx,
		binaryPath,
		server.ServeArgs(ep.Network, ep.Address)...,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = shutdownTimeout
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

// isOrdinaryStop reports whether err is one of the ways a validator we asked to stop is
// supposed to go down. What is left is a process the runtime could not signal or reap —
// the only thing a caller can act on.
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
