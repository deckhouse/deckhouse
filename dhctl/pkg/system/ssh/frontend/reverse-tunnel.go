// Copyright 2024 Flant JSC
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
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type ReverseTunnel struct {
	Session *session.Session
	Address string
	sshCmd  *exec.Cmd
	sshCl   *ssh.Client

	tunMutex sync.Mutex
	stopped  bool
	id       int

	stopCh  chan struct{}
	errorCh chan error

	port int
}

func NewReverseTunnel(sess *session.Session, address string, cl *ssh.Client) *ReverseTunnel {
	addressPart := strings.Split(address, ":")
	port, err := strconv.Atoi(addressPart[len(addressPart)-1])
	if err != nil {
		panic(fmt.Sprintf("Cannot parse tunnel port %v", err))
	}
	return &ReverseTunnel{
		Session: sess,
		Address: address,
		errorCh: make(chan error),
		port:    port,
		sshCl:   cl,
	}
}

func (t *ReverseTunnel) Up(id int) error {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()

	t.stopped = false

	if id < 0 {
		id = rand.Int()
		t.id = id
	}

	log.DebugF("[%d] Start reverse tunnel\n", t.id)
	defer log.DebugF("[%d] End start reverse tunnel\n", t.id)

	if t.Session == nil {
		return fmt.Errorf("[%d] up tunnel '%s': SSH client is undefined", t.id, t.String())
	}

	t.sshCmd = cmd.NewSSH(t.Session).
		WithArgs(
			"-N", // no command
			"-n", // no stdin
			"-R", t.Address,
		).
		WithExitWhenTunnelFailure(true).
		Cmd()

	err := t.sshCmd.Start()
	if err != nil {
		return fmt.Errorf("[%d] tunnel up: %w", t.id, err)
	}

	go func() {
		log.DebugF("[%d] Reverse tunnel started. Wait stop tunnel...\n", t.id)
		t.errorCh <- t.sshCmd.Wait()
		log.DebugF("[d] Reverse tunnel was stopped\n", t.id)
	}()

	return nil
}

func (t *ReverseTunnel) getId() int {
	t.tunMutex.Lock()
	i := t.id
	t.tunMutex.Unlock()

	return i
}

func (t *ReverseTunnel) StartHealthMonitor() {
	t.tunMutex.Lock()
	t.stopCh = make(chan struct{})
	t.tunMutex.Unlock()

	go func() {
		id := t.getId()
		log.DebugF("[%d] Health monitor start\n", id)

		file, err := template.RenderAndSavePreflightReverseTunnelOpenScript(t.port)
		if err != nil {
			panic(fmt.Sprintf("Cannot render reverse tunnel script: %v", err))
		}

		// buffered chanel for none blocking continue
		restartCh := make(chan struct{}, 1)

		restartTunnel := func(rCh chan struct{}) {
			// try restart fully
			err := t.stop(false)
			if err != nil {
				log.DebugF("[%d] Error stopping tunnel: %s", id, err)
			}

			newId := rand.Int()

			err = t.Up(newId)
			if err != nil {
				log.DebugF("[%d] Cannot up new tunnel: %d\n", err)
				if rCh != nil {
					rCh <- struct{}{}
				}
			}
		}

		checkReverseTunnel := func() bool {
			scriptCmd := t.sshCl.UploadScript(file)
			out, err := scriptCmd.Execute()
			if err != nil {
				log.DebugF("Cannot check ssh tunnel: '%v': \n", err, string(out))
				restartTunnel(nil)
				return false
			}

			return true
		}

		for {
			if !checkReverseTunnel() {
				continue
			}

			select {
			case <-t.stopCh:
				id := t.getId()
				log.DebugF("[%d] Health monitor stopped\n", id)
				return
			case <-restartCh:
				id := t.getId()
				log.DebugF("[%d] Tunnel was not up '%s'. Try restart fully\n", id)
				restartTunnel(restartCh)
			case err := <-t.errorCh:
				id := t.getId()
				log.DebugF("[%d] Tunnel was stopped with error '%s'. Try restart fully\n", id, err)
				restartTunnel(restartCh)
			}
		}
	}()
}

func (t *ReverseTunnel) Stop() error {
	return t.stop(true)
}

func (t *ReverseTunnel) stop(gracefully bool) error {
	if t.stopped {
		log.DebugLn("Reverse tunnel already stopped")
		return nil
	}

	id := t.getId()

	log.DebugF("[%d] Stop reverse tunnel\n", id)
	defer log.DebugLn("[%d] End stop reverse tunnel\n", id)

	if gracefully && t.stopCh != nil {
		t.stopCh <- struct{}{}
	}

	err := t.sshCmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("[%d] stop tunnel '%s': %w", id, t.String(), err)
	}

	t.tunMutex.Lock()
	t.stopped = true
	t.tunMutex.Unlock()

	return nil
}

func (t *ReverseTunnel) String() string {
	return fmt.Sprintf("%s:%s", "R", t.Address)
}
