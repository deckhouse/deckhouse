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

package frontend

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

type Tunnel struct {
	Session *session.Session
	Type    string // Remote or Local
	Address string
	sshCmd  *exec.Cmd

	pipesMutex      sync.Mutex
	stdoutReadPipe  *os.File
	stdoutWritePipe *os.File
	stdinReadPipe   *os.File
	stdinWritePipe  *os.File

	stopOnce sync.Once
	stopped  atomic.Bool
	stopCh   chan struct{}
	errorCh  chan error
}

type tunnelPipes struct {
	stdoutReadPipe  *os.File
	stdoutWritePipe *os.File
	stdinReadPipe   *os.File
	stdinWritePipe  *os.File
}

func NewTunnel(sess *session.Session, ttype, address string) *Tunnel {
	return &Tunnel{
		Session: sess,
		Type:    ttype,
		Address: address,
		stopCh:  make(chan struct{}, 1),
		errorCh: make(chan error, 1),
	}
}

func (t *Tunnel) Up() error {
	return t.UpContext(context.Background())
}

func (t *Tunnel) UpContext(ctx context.Context) error {
	if t.Session == nil {
		return fmt.Errorf("up tunnel '%s': SSH client is undefined", t.String())
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	t.stopOnce = sync.Once{}
	t.stopped.Store(false)
	t.stopCh = make(chan struct{}, 1)
	t.errorCh = make(chan error, 1)

	t.sshCmd = cmd.NewSSH(t.Session).
		WithArgs(
			// "-f", // start in background - good for scripts, but here we need to do cmd.Process.Kill()
			"-o", "ExitOnForwardFailure=yes", // wait for connection establish before
			// "-N",                       // no command
			// "-n", // no stdin
			fmt.Sprintf("-%s", t.Type), t.Address,
		).
		WithCommand("echo", "SUCCESS", "&&", "cat").
		Cmd(ctx)

	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdout: %w", err)
	}
	t.sshCmd.Stdout = stdoutWritePipe
	t.pipesMutex.Lock()
	t.stdoutReadPipe = stdoutReadPipe
	t.stdoutWritePipe = stdoutWritePipe
	t.pipesMutex.Unlock()

	// Create separate stdin pipe to prevent reading from main process Stdin
	stdinReadPipe, stdinWritePipe, err := os.Pipe()
	if err != nil {
		t.closePipes()
		return fmt.Errorf("unable to create os pipe for stdin: %w", err)
	}
	t.sshCmd.Stdin = stdinReadPipe
	t.pipesMutex.Lock()
	t.stdinReadPipe = stdinReadPipe
	t.stdinWritePipe = stdinWritePipe
	t.pipesMutex.Unlock()

	if err = ctx.Err(); err != nil {
		t.closePipes()
		return err
	}

	err = t.sshCmd.Start()
	if err != nil {
		t.closePipes()
		return fmt.Errorf("tunnel up: %w", err)
	}

	tunnelReadyCh := make(chan struct{}, 1)
	go func() {
		// defer wg.Done()
		t.consumeLines(stdoutReadPipe, func(l string) {
			if l == "SUCCESS" {
				tunnelReadyCh <- struct{}{}
			}
		})
		log.DebugF("stop line consumer for '%s'", t.String())
	}()

	sshCmd := t.sshCmd
	errorCh := t.errorCh
	pipes := tunnelPipes{
		stdoutReadPipe:  stdoutReadPipe,
		stdoutWritePipe: stdoutWritePipe,
		stdinReadPipe:   stdinReadPipe,
		stdinWritePipe:  stdinWritePipe,
	}
	t.waitForSSH(sshCmd, errorCh, pipes)

	select {
	case err = <-errorCh:
		t.closePipes()
		return fmt.Errorf("cannot open tunnel '%s': %w", t.String(), err)
	case <-tunnelReadyCh:
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	}

	return nil
}

func (t *Tunnel) waitForSSH(sshCmd *exec.Cmd, errorCh chan<- error, pipes tunnelPipes) {
	go func() {
		err := sshCmd.Wait()
		t.closePipeSet(pipes)
		errorCh <- err
	}()
}

func (t *Tunnel) HealthMonitor(errorOutCh chan<- error) {
	defer log.DebugF("Tunnel health monitor stopped\n")
	log.DebugF("Tunnel health monitor started\n")

	for {
		select {
		case err := <-t.errorCh:
			if t.stopped.Load() {
				return
			}
			select {
			case errorOutCh <- err:
			case <-t.stopCh:
				return
			}
		case <-t.stopCh:
			return
		}
	}
}

func (t *Tunnel) Stop() {
	if t == nil {
		return
	}
	t.stopOnce.Do(func() {
		t.stopped.Store(true)
		t.closePipes()
		t.killProcess()
		t.signalStop()

		if t.Session == nil {
			log.ErrorF("bug: down tunnel '%s': no session", t.String())
			return
		}
	})
}

func (t *Tunnel) signalStop() {
	if t.stopCh == nil {
		return
	}

	select {
	case t.stopCh <- struct{}{}:
	default:
	}
}

func (t *Tunnel) killProcess() {
	if t.sshCmd == nil || t.sshCmd.Process == nil {
		return
	}

	err := t.sshCmd.Process.Kill()
	if err != nil {
		log.DebugF("Cannot kill tunnel process %d: %v\n", t.sshCmd.Process.Pid, err)
	}
}

func (t *Tunnel) closePipes() {
	t.pipesMutex.Lock()
	defer t.pipesMutex.Unlock()

	t.closePipeSet(tunnelPipes{
		stdoutReadPipe:  t.stdoutReadPipe,
		stdoutWritePipe: t.stdoutWritePipe,
		stdinReadPipe:   t.stdinReadPipe,
		stdinWritePipe:  t.stdinWritePipe,
	})
	t.stdoutReadPipe = nil
	t.stdoutWritePipe = nil
	t.stdinReadPipe = nil
	t.stdinWritePipe = nil
}

func (t *Tunnel) closePipeSet(pipes tunnelPipes) {
	t.closePipeFile(pipes.stdoutReadPipe, "stdout read pipe")
	t.closePipeFile(pipes.stdoutWritePipe, "stdout write pipe")
	t.closePipeFile(pipes.stdinReadPipe, "stdin read pipe")
	t.closePipeFile(pipes.stdinWritePipe, "stdin write pipe")
}

func (t *Tunnel) closePipeFile(pipe *os.File, name string) {
	if pipe == nil {
		return
	}

	err := pipe.Close()
	if err != nil {
		log.DebugF("Cannot close tunnel %s: %v\n", name, err)
	}
}

func (t *Tunnel) String() string {
	return fmt.Sprintf("%s:%s", t.Type, t.Address)
}

func (t *Tunnel) consumeLines(r io.Reader, fn func(l string)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()

		if fn != nil {
			fn(text)
		}
	}
}
