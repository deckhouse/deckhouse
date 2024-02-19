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
	"strings"
	"syscall"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type SSH struct {
	*process.Executor
	Session     *session.Session
	Args        []string
	Env         []string
	CommandName string
	CommandArgs []string
}

func NewSSH(sess *session.Session) *SSH {
	return &SSH{Session: sess}
}

func (s *SSH) WithEnv(env ...string) *SSH {
	s.Env = env
	return s
}

func (s *SSH) WithArgs(args ...string) *SSH {
	s.Args = args
	return s
}

func (s *SSH) WithCommand(name string, arg ...string) *SSH {
	s.CommandName = name
	s.CommandArgs = arg
	return s
}

// TODO move connection settings from ExecuteCmd
func (s *SSH) Cmd() *exec.Cmd {
	env := append(os.Environ(), s.Env...)
	env = append(env, s.Session.AgentSettings.AuthSockEnv())

	// ssh connection settings
	//   ANSIBLE_SSH_ARGS="${ANSIBLE_SSH_ARGS:-"-C
	//   -o ControlMaster=auto
	//  -o ControlPersist=600s"}
	args := []string{
		// ssh args for bastion here
		"-C", // compression
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=600s",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "GlobalKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "ConnectTimeout=15",
		"-o", "PasswordAuthentication=no",
	}

	if app.IsDebug {
		args = append(args, "-vvv")
	}

	if s.Session.ExtraArgs != "" {
		extraArgs := strings.Split(s.Session.ExtraArgs, " ")
		if len(extraArgs) > 0 {
			args = append(args, extraArgs...)
		}
	}

	if len(s.Args) > 0 {
		args = append(args, s.Args...)
	}

	// add bastion options
	//  if [[ "x$ssh_bastion_host" != "x" ]] ; then
	//    export ANSIBLE_SSH_ARGS="${ANSIBLE_SSH_ARGS}
	//   -o ProxyCommand='ssh ${ssh_bastion_user:-$USER}@$ssh_bastion_host -W %h:%p'"
	//  fi
	if s.Session.BastionHost != "" {
		bastion := s.Session.BastionHost
		if s.Session.BastionUser != "" {
			bastion = s.Session.BastionUser + "@" + s.Session.BastionHost
		}
		if s.Session.BastionPort != "" {
			bastion = bastion + " -p" + s.Session.BastionPort
		}
		args = append(args, []string{
			// 1. Note that single quotes is not needed here
			// 2. Add all arguments to the proxy command so the connection to bastion has the same args
			"-o", fmt.Sprintf("ProxyCommand=ssh %s -W %%h:%%p %s", bastion, strings.Join(args, " ")),
			"-o", "ExitOnForwardFailure=yes",
		}...)
	}

	// add destination: user, host and port
	if s.Session.User != "" {
		args = append(args, []string{
			"-l",
			s.Session.User,
		}...)
	}
	if s.Session.Port != "" {
		args = append(args, []string{
			"-p",
			s.Session.Port,
		}...)
	}

	args = append(args, s.Session.Host())

	if s.CommandName != "" {
		args = append(args, "--" /* cmd.Path */, s.CommandName)
		args = append(args, s.CommandArgs...)
	}

	log.DebugF("SSH arguments %v\n", args)

	sshCmd := exec.Command("ssh", args...)
	sshCmd.Env = env
	// Start ssh with the new process group to prevent early stop by SIGINT from the shell.
	sshCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	s.Executor = process.NewDefaultExecutor(sshCmd)

	return sshCmd
}
