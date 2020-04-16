package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Ssh struct {
	SshClient   *SshClient
	Args        []string
	Env         []string
	CommandName string
	CommandArgs []string
}

func (s *Ssh) WithEnv(env ...string) *Ssh {
	s.Env = env
	return s
}

func (s *Ssh) WithArgs(args ...string) *Ssh {
	s.Args = args
	return s
}

func (s *Ssh) WithCommand(name string, arg ...string) *Ssh {
	s.CommandName = name
	s.CommandArgs = arg
	return s
}

// TODO move connection settings from ExecuteCmd
func (s *Ssh) Cmd() *exec.Cmd {
	env := append(os.Environ(), s.Env...)
	env = append(env, fmt.Sprintf("SSH_AUTH_SOCK=%s", s.SshClient.SshAgentAuthSock))

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
		"-o",
		"ControlMaster=auto",
		"-o",
		"ControlPersist=600s",
		"-o",
		"StrictHostKeyChecking=accept-new",
		"-o",
		"UserKnownHostsFile=.ssh_known_hosts",
	}

	if s.SshClient.ExtraArgs != "" {
		extraArgs := strings.Split(s.SshClient.ExtraArgs, " ")
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
	if s.SshClient.BastionHost != "" {
		bastion := s.SshClient.BastionHost
		if s.SshClient.BastionUser != "" {
			bastion = s.SshClient.BastionUser + "@" + s.SshClient.BastionHost
		}
		args = append(args, []string{
			"-o",
			fmt.Sprintf("ProxyCommand=ssh %s -W %%h:%%p", bastion), // note that single quotes is not needed here
		}...)
	}

	// add destination: user and host
	if s.SshClient.User != "" {
		args = append(args, []string{
			"-l",
			s.SshClient.User,
		}...)
	}

	args = append(args, s.SshClient.Host)

	if s.CommandName != "" {
		// Add command and arguments
		args = append(args, "--")
		//args = append(args, cmd.Path)
		args = append(args, s.CommandName)
		args = append(args, s.CommandArgs...)
	}

	sshCmd := exec.Command("ssh", args...)
	sshCmd.Env = env

	return sshCmd
}
