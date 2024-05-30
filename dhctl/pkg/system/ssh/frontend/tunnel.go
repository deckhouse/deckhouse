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
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type Tunnel struct {
	Session *session.Session
	Type    string // Remote or Local
	Address string
	sshCmd  *exec.Cmd

	stopCh  chan struct{}
	errorCh chan error
}

func NewTunnel(sess *session.Session, ttype, address string) *Tunnel {
	return &Tunnel{
		Session: sess,
		Type:    ttype,
		Address: address,
		errorCh: make(chan error, 1),
	}
}

func (t *Tunnel) Up() error {
	if t.Session == nil {
		return fmt.Errorf("up tunnel '%s': SSH client is undefined", t.String())
	}

	t.sshCmd = cmd.NewSSH(t.Session).
		WithArgs(
			// "-f", // start in background - good for scripts, but here we need to do cmd.Process.Kill()
			"-o", "ExitOnForwardFailure=yes", // wait for connection establish before
			// "-N",                       // no command
			// "-n", // no stdin
			fmt.Sprintf("-%s", t.Type), t.Address,
		).
		WithCommand("echo", "SUCCESS", "&&", "cat").
		Cmd()

	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdout: %w", err)
	}
	t.sshCmd.Stdout = stdoutWritePipe

	// Create separate stdin pipe to prevent reading from main process Stdin
	stdinReadPipe, _, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdin: %w", err)
	}
	t.sshCmd.Stdin = stdinReadPipe

	err = t.sshCmd.Start()
	if err != nil {
		return fmt.Errorf("tunnel up: %w", err)
	}

	tunnelReadyCh := make(chan struct{}, 1)
	go func() {
		// defer wg.Done()
		t.ConsumeLines(stdoutReadPipe, func(l string) {
			if l == "SUCCESS" {
				tunnelReadyCh <- struct{}{}
			}
		})
		log.DebugF("stop line consumer for '%s'", t.String())
	}()

	go func() {
		t.errorCh <- t.sshCmd.Wait()
	}()

	select {
	case err = <-t.errorCh:
		return fmt.Errorf("cannot open tunnel '%s': %w", t.String(), err)
	case <-tunnelReadyCh:
	}

	return nil
}

func (t *Tunnel) HealthMonitor(errorOutCh chan<- error) {
	defer log.DebugF("Tunnel health monitor stopped\n")
	log.DebugF("Tunnel health monitor started\n")

	t.stopCh = make(chan struct{}, 1)

	for {
		select {
		case err := <-t.errorCh:
			errorOutCh <- err
		case <-t.stopCh:
			_ = t.sshCmd.Process.Kill()
			return
		}
	}
}

func (t *Tunnel) Stop() {
	if t == nil {
		return
	}
	if t.Session == nil {
		log.ErrorF("bug: down tunnel '%s': no session", t.String())
		return
	}

	if t.sshCmd != nil && t.stopCh != nil {
		t.stopCh <- struct{}{}
	}
}

func (t *Tunnel) String() string {
	return fmt.Sprintf("%s:%s", t.Type, t.Address)
}

func (t *Tunnel) ConsumeLines(r io.Reader, fn func(l string)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()

		if fn != nil {
			fn(text)
		}
	}
}
