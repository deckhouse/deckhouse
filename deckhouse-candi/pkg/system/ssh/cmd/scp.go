package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"flant/deckhouse-candi/pkg/system/process"
	"flant/deckhouse-candi/pkg/system/ssh/session"
)

type Scp struct {
	*process.Executor

	Session *session.Session
	scpCmd  *exec.Cmd

	RemoteDst bool
	Dst       string
	RemoteSrc bool
	Src       string
	Preserve  bool
	Recursive bool
}

func NewScp(sess *session.Session) *Scp {
	return &Scp{Session: sess}
}

func (s *Scp) WithRemoteDst(path string) *Scp {
	s.RemoteDst = true
	s.Dst = path
	return s
}
func (s *Scp) WithDst(path string) *Scp {
	s.RemoteDst = false
	s.Dst = path
	return s
}
func (s *Scp) WithRemoteSrc(path string) *Scp {
	s.RemoteSrc = true
	s.Src = path
	return s
}
func (s *Scp) WithSrc(path string) *Scp {
	s.RemoteSrc = false
	s.Src = path
	return s
}

func (s *Scp) WithRecursive(recursive bool) *Scp {
	s.Recursive = recursive
	return s
}

func (s *Scp) WithPreserve(preserve bool) *Scp {
	s.Preserve = preserve
	return s
}

func (s *Scp) Scp() *Scp {
	//env := append(os.Environ(), s.Env...)
	env := append(os.Environ(), s.Session.AuthSockEnv())

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

	if s.Session.ExtraArgs != "" {
		extraArgs := strings.Split(s.Session.ExtraArgs, " ")
		if len(extraArgs) > 0 {
			args = append(args, extraArgs...)
		}
	}

	// add bastion options
	if s.Session.BastionHost != "" {
		bastion := s.Session.BastionHost
		if s.Session.BastionUser != "" {
			bastion = s.Session.BastionUser + "@" + s.Session.BastionHost
		}
		if s.Session.BastionPort != "" {
			bastion = bastion + " -p" + s.Session.BastionPort
		}
		args = append(args, []string{
			"-o",
			fmt.Sprintf("ProxyCommand=ssh %s -W %%h:%%p", bastion), // note that single quotes is not needed here
			"-o",
			"ExitOnForwardFailure=yes",
		}...)
	}

	// add remote port if defined
	if s.Session.Port != "" {
		args = append(args, []string{
			"-P",
			s.Session.Port,
		}...)
	}

	if s.Preserve {
		args = append(args, "-p")
	}

	if s.Recursive {
		args = append(args, "-r")
	}

	// create src path
	srcPath := s.Src
	if s.RemoteSrc {
		srcPath = s.Session.RemoteAddress() + ":" + srcPath
	}

	//create dest path
	dstPath := s.Dst
	if dstPath == "" {
		dstPath = "."
	}
	if !strings.HasPrefix(dstPath, "/") && !strings.HasPrefix(dstPath, ".") {
		dstPath = "./" + dstPath
	}
	if s.RemoteDst {
		dstPath = s.Session.RemoteAddress() + ":" + dstPath
	}

	args = append(args, []string{
		srcPath,
		dstPath,
	}...)

	s.scpCmd = exec.Command("scp", args...)
	s.scpCmd.Env = env
	// scpCmd.Stdout = os.Stdout
	// scpCmd.Stderr = os.Stderr

	s.Executor = process.NewDefaultExecutor(s.scpCmd)

	return s
}
