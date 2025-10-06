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

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const SSHAgentPath = "ssh-agent"

type SSHAgent struct {
	*process.Executor

	AgentSettings *session.AgentSettings

	agentCmd *exec.Cmd

	Pid      string
	AuthSock string
}

var SSHAgentAuthSockRe = regexp.MustCompile(`SSH_AUTH_SOCK=(.*?);`)

// Start runs ssh-agent as a subprocess, gets SSH_AUTH_SOCK path and
func (a *SSHAgent) Start() error {
	a.agentCmd = exec.Command(SSHAgentPath, "-D")
	a.agentCmd.Env = os.Environ()
	a.agentCmd.Dir = "/"
	// Start ssh-agent with the new session to prevent terminal allocation and early stop by SIGINT.
	a.agentCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	a.Executor = process.NewDefaultExecutor(a.agentCmd)
	// a.EnableLive()
	a.WithStdoutHandler(func(l string) {
		log.DebugF("ssh agent: got '%s'\n", l)

		m := SSHAgentAuthSockRe.FindStringSubmatch(l)
		if len(m) == 2 && m[1] != "" {
			a.AuthSock = m[1]
		}
	})

	a.WithWaitHandler(func(err error) {
		if err != nil {
			log.ErrorF("SSH-agent process exited, now stop. Wait error: %v\n", err)
			return
		}
		log.InfoF("SSH-agent process exited, now stop.\n")
	})

	err := a.Executor.Start()
	if err != nil {
		a.agentCmd = nil
		return fmt.Errorf("start ssh-agent subprocess: %v", err)
	}

	// wait for ssh agent pid
	success := false
	maxWait := 1000
	retries := 0
	t := time.NewTicker(5 * time.Millisecond)
	for {
		<-t.C
		if a.AuthSock != "" {
			log.DebugF("ssh-agent: SSH_AUTH_SOCK=%s\n", a.AuthSock)
			success = true
			break
		}
		retries++
		if retries > maxWait {
			break
		}
	}
	t.Stop()

	if !success {
		a.Stop()
		return fmt.Errorf("cannot get pid and auth sock path for ssh-agent")
	}

	// save auth sock in session to access it from other cmds and frontends
	a.AgentSettings.AuthSock = a.AuthSock
	tomb.RegisterOnShutdown("Delete SSH agent temporary directory", func() {
		_ = os.RemoveAll(filepath.Dir(a.AuthSock))
	})
	return nil
}

func (a *SSHAgent) Stop() {
	if a.Executor != nil {
		a.Executor.Stop()
	}
}
