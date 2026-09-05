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
	"log/slog"
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
	binaryPath    string
	endpoint      endpoint
	stdoutHandler lineHandler
	stderrHandler lineHandler
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
	opt    validatorOptions
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelCauseFunc
	cmd    *exec.Cmd

	stdout *lineWriter
	stderr *lineWriter

	waitWG  sync.WaitGroup
	waitErr error

	stopOnce sync.Once
	stopErr  error
}

func newValidatorProcess(ctx context.Context, opt validatorOptions) (*validatorProcess, error) {
	if err := opt.validate(); err != nil {
		return nil, fmt.Errorf("validator options: %w", err)
	}

	return &validatorProcess{
		opt:    opt,
		logger: dhlog.FromContext(ctx),
	}, nil
}

// Start spawns the validator and returns the context of its life: it is cancelled when
// the process goes down, so a request made with it fails the moment the validator dies
// instead of waiting out its own deadline.
func (v *validatorProcess) Start(ctx context.Context) (context.Context, error) {
	ctx, cancel := context.WithCancelCause(ctx)

	v.ctx = ctx
	v.cancel = cancel

	v.stdout = newLineWriter(mergeLineHandlers(v.debug, v.opt.stdoutHandler))
	v.stderr = newLineWriter(mergeLineHandlers(v.debug, v.opt.stderrHandler))

	v.cmd = validatorCmd(ctx, v.opt.binaryPath, v.opt.endpoint, v.stdout, v.stderr)

	v.debug(fmt.Sprintf("start: %s", v.cmd.String()))
	if err := v.cmd.Start(); err != nil {
		if stopErr := v.Stop(); stopErr != nil {
			return nil, errors.Join(err, stopErr)
		}

		return nil, err
	}

	v.waitWG.Go(func() {
		v.waitErr = v.cmd.Wait()
		_ = syscall.Kill(-v.cmd.Process.Pid, syscall.SIGKILL)
		v.cancel(fmt.Errorf("validator exited: %w", v.waitErr))
		v.debug(fmt.Sprintf("validator exited: %v", v.waitErr))
	})

	return ctx, nil
}

// Stop is safe to call multiple times and reports the same result every time. The wait
// is bounded by cmd.WaitDelay: cancelling starts its timer, and when it expires
// os/exec kills the child and closes its pipes.
func (v *validatorProcess) Stop() error {
	if v.cancel == nil {
		return nil
	}

	v.stopOnce.Do(func() {
		v.debug("stop validator")

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

func (v *validatorProcess) debug(line string) {
	v.logger.Debug(fmt.Sprintf("validator: %s", line))
}

// listeningValidator is a validator plus the endpoint it announced: the caller asks for
// port 0, so where it listens is known only once it says so.
type listeningValidator struct {
	*validatorProcess

	announcedCh chan endpoint
	endpoint    endpoint
}

func newListeningValidator(ctx context.Context, binaryPath string) (*listeningValidator, error) {
	ret := &listeningValidator{
		announcedCh: make(chan endpoint, 1),
	}

	process, err := newValidatorProcess(ctx, validatorOptions{
		binaryPath: binaryPath,
		endpoint: endpoint{
			Network: networkTCP,
			Address: loopbackAddress,
		},
		stdoutHandler: func(line string) {
			ret.catchEndpoint(line)
		},
	})
	if err != nil {
		return nil, err
	}

	ret.validatorProcess = process
	return ret, nil
}

// Start spawns the validator and waits for it to say where it listens, so the caller
// gets a process it can talk to right away.
func (v *listeningValidator) Start(ctx context.Context) (context.Context, error) {
	ctx, err := v.validatorProcess.Start(ctx)
	if err != nil {
		return nil, err
	}

	if err := v.waitEndpoint(ctx); err != nil {
		if stopErr := v.Stop(); stopErr != nil {
			return nil, errors.Join(err, stopErr)
		}

		return nil, err
	}

	return ctx, nil
}

func (v *listeningValidator) Endpoint() endpoint {
	return v.endpoint
}

func (v *listeningValidator) catchEndpoint(line string) {
	network, address, ok := server.ParseListeningLine(line)
	if !ok {
		return
	}

	select {
	case v.announcedCh <- endpoint{Network: network, Address: address}:
	default:
	}
}

// waitEndpoint blocks until the validator announces where it listens. Whichever way the
// wait ends, a process that is gone is reported as gone: an endpoint from a validator
// that has already died is worth nothing.
func (v *listeningValidator) waitEndpoint(ctx context.Context) error {
	deadline, cancel := context.WithTimeout(ctx, announceTimeout)
	defer cancel()

	select {
	case announced := <-v.announcedCh:
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}

		v.endpoint = announced
		return nil

	case <-deadline.Done():
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		return fmt.Errorf("announced no endpoint within %s", announceTimeout)
	}
}

func validatorCmd(ctx context.Context, binaryPath string, ep endpoint, stdout, stderr io.Writer) *exec.Cmd {
	cmd := exec.CommandContext(
		ctx,
		binaryPath,
		server.ServeArgs(ep.Network, ep.Address)...,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
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
