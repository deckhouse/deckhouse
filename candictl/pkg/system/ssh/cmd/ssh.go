package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/deckhouse/deckhouse/candictl/pkg/system/process"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh/session"
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
	env = append(env, s.Session.AuthSockEnv())

	// ssh connection settings
	//   ANSIBLE_SSH_ARGS="${ANSIBLE_SSH_ARGS:-"-C
	//   -o ControlMaster=auto
	//  -o ControlPersist=600s"}
	//
	// -o StrictHostKeyChecking=accept-new
	// -o UserKnownHostsFile=$(pwd)/.konverge/$terraform_workspace/.ssh_known_hosts"
	args := []string{
		// ssh args for bastion here
		"-C", // compression
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=600s",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=.ssh_known_hosts",
		"-o", "ServerAliveInterval=15",
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
			"-o", fmt.Sprintf("ProxyCommand=ssh %s -W %%h:%%p", bastion), // note that single quotes is not needed here
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

	args = append(args, s.Session.Host)

	if s.CommandName != "" {
		args = append(args, "--" /* cmd.Path */, s.CommandName)
		args = append(args, s.CommandArgs...)
	}

	sshCmd := exec.Command("ssh", args...)
	sshCmd.Env = env

	s.Executor = process.NewDefaultExecutor(sshCmd)

	return sshCmd
}
