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
	"os/exec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type ReverseTunnel struct {
	Session *session.Session
	Address string
	sshCmd  *exec.Cmd
}

func NewReverseTunnel(sess *session.Session, address string) *ReverseTunnel {
	return &ReverseTunnel{
		Session: sess,
		Address: address,
	}
}

func (t *ReverseTunnel) Up() error {
	if t.Session == nil {
		return fmt.Errorf("up tunnel '%s': SSH client is undefined", t.String())
	}

	t.sshCmd = cmd.NewSSH(t.Session).
		WithArgs(
			"-N", // no command
			"-n", // no stdin
			"-R", t.Address,
		).
		Cmd()

	err := t.sshCmd.Start()
	if err != nil {
		return fmt.Errorf("tunnel up: %v", err)
	}

	go func() {
		err = t.sshCmd.Wait()
		if err != nil {
			log.ErrorF("cannot open tunnel '%s': %v", t.String(), err)
		}
	}()

	return nil
}

func (t *ReverseTunnel) Stop() error {
	err := t.sshCmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("stop tunnel '%s': %v", t.String(), err)
	}

	return nil
}

func (t *ReverseTunnel) String() string {
	return fmt.Sprintf("%s:%s", "R", t.Address)
}
