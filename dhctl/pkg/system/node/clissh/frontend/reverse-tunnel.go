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
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

type tunnelWaitResult struct {
	id  int
	err error
}

type ReverseTunnel struct {
	Session *session.Session
	Address string

	tunMutex sync.Mutex
	sshCmd   *exec.Cmd
	started  bool
	stopCh   chan struct{}

	errorCh chan tunnelWaitResult
}

func NewReverseTunnel(sess *session.Session, address string) *ReverseTunnel {
	return &ReverseTunnel{
		Session: sess,
		Address: address,
		errorCh: make(chan tunnelWaitResult),
	}
}

func (t *ReverseTunnel) Up() error {
	_, err := t.upNewTunnel(-1)
	return err
}

func (t *ReverseTunnel) upNewTunnel(oldId int) (int, error) {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()

	if t.started {
		log.DebugF("[%d] Reverse tunnel already up\n", oldId)
		return -1, fmt.Errorf("already up")
	}

	id := rand.Int()

	log.DebugF("[%d] Start reverse tunnel\n", id)

	t.sshCmd = cmd.NewSSH(t.Session).
		WithArgs(
			"-N", // no command
			"-n", // no stdin
			"-R", t.Address,
		).
		WithExitWhenTunnelFailure(true).
		Cmd(context.Background())

	err := t.sshCmd.Start()
	if err != nil {
		return id, fmt.Errorf("[%d] Cannot start tunnel ssh command: %w", id, err)
	}

	go func(localCmd *exec.Cmd, localID int) {
		if localCmd == nil {
			log.ErrorF("[%d] sshCmd is nil before Wait()\n", localID)

			t.errorCh <- tunnelWaitResult{
				id:  localID,
				err: fmt.Errorf("cannot Wait(): sshCmd is nil"),
			}

			return
		}

		log.DebugF("[%d] Reverse tunnel started. Waiting for tunnel to stop...\n", localID)

		err := localCmd.Wait()

		t.errorCh <- tunnelWaitResult{
			id:  localID,
			err: err,
		}

		log.DebugF("[%d] Reverse tunnel was stopped and handled\n", localID)
	}(t.sshCmd, id)

	t.started = true

	return id, nil
}

func (t *ReverseTunnel) isStarted() bool {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()
	r := t.started
	return r
}

func (t *ReverseTunnel) tryToRestart(ctx context.Context, id int, killer node.ReverseTunnelKiller) (int, error) {
	t.stop(id, false)
	log.DebugF("[%d] Kill tunnel\n", id)
	if out, err := killer.KillTunnel(ctx); err != nil {
		log.DebugF("[%d] Kill tunnel was finished with error: %v; stdout: '%s'\n", id, err, out)
		return id, err
	}
	return t.upNewTunnel(id)
}

func (t *ReverseTunnel) StartHealthMonitor(ctx context.Context, checker node.ReverseTunnelChecker, killer node.ReverseTunnelKiller) {
	t.tunMutex.Lock()
	t.stopCh = make(chan struct{})
	t.tunMutex.Unlock()

	checkReverseTunnel := func(id int) bool {
		log.DebugF("[%d] Start Check reverse tunnel\n", id)

		err := retry.NewSilentLoop("Check reverse tunnel", 2, 2*time.Second).RunContext(ctx, func() error {
			out, err := checker.CheckTunnel(ctx)
			if err != nil {
				log.DebugF("[%d] Cannot check ssh tunnel: '%v': stderr: '%s'\n", id, err, out)
				return err
			}

			return nil
		})

		if err != nil {
			log.DebugF("[%d] Tunnel check timeout, last error: %v\n", id, err)
			return false
		}

		log.DebugF("[%d] Tunnel check successful!\n", id)
		return true
	}

	go func() {
		log.DebugLn("Start health monitor")
		// we need chan for restarting because between restarting we can get stop signal
		restartCh := make(chan int, 1024)
		id := -1
		restartsCount := 0
		restart := func(id int) {
			log.DebugF("[%d] Send restart signal\n", id)
			restartCh <- id
			log.DebugF("[%d] Signal was sent. Chan len: %d\n", id, len(restartCh))
		}
		for {

			if !checkReverseTunnel(id) {
				go restart(id)
			}

			select {
			case <-t.stopCh:
				log.DebugLn("Stop health monitor")
				return
			case oldId := <-restartCh:
				restartsCount++
				log.DebugF("[%d] Restart signal was received: restarts count %d\n", oldId, restartsCount)

				if restartsCount > 1024 {
					panic("Reverse tunnel restarts count exceeds 1024")
				}

				newId, err := t.tryToRestart(ctx, oldId, killer)
				if err != nil {
					log.DebugF("[%d] Restart failed with error: %v\n", oldId, err)
					go restart(oldId)
					continue
				}
				log.DebugF("[%d] Restart successful. New id %d\n", oldId, newId)
				id = newId
				restartsCount = 0
			case err := <-t.errorCh:
				id = err.id
				log.DebugF("[%d] Tunnel was stopped with error '%v'. Try restart fully\n", id, err.err)
				started := t.isStarted()
				if started {
					log.DebugF("[%d] Tunnel already up. Skip restarting\n", id)
					continue
				}

				go restart(id)
				continue
			}
		}
	}()
}

func (t *ReverseTunnel) Stop() {
	t.stop(-1, true)
}

func (t *ReverseTunnel) stop(id int, full bool) {
	t.tunMutex.Lock()
	defer t.tunMutex.Unlock()

	if !t.started {
		log.DebugF("[%d] Reverse tunnel already stopped\n", id)
		return
	}

	log.DebugF("[%d] Stop reverse tunnel\n", id)
	defer log.DebugF("[%d] End stop reverse tunnel\n", id)

	if full && t.stopCh != nil {
		log.DebugF("[%d] Stop reverse tunnel health monitor\n", id)
		t.stopCh <- struct{}{}
	}

	log.DebugF("[%d] Try to find tunnel process %d\n", id, t.sshCmd.Process.Pid)
	_, err := os.FindProcess(t.sshCmd.Process.Pid)
	if err == nil {
		log.DebugF("[%d] Process found %d. Kill it\n", id, t.sshCmd.Process.Pid)
		err := t.sshCmd.Process.Kill()
		if err != nil {
			log.DebugF("[%d] Cannot kill process %d: %v\n", id, t.sshCmd.Process.Pid, err)
		}
	} else {
		log.DebugF("[%d] Stopping tunnel. Process %d already finished\n", id, t.sshCmd.Process.Pid)
	}

	t.sshCmd = nil
	t.started = false
}

func (t *ReverseTunnel) String() string {
	return fmt.Sprintf("%s:%s", "R", t.Address)
}
