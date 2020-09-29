package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh/session"
)

var SshAddPath = "ssh-add"

type SshAdd struct {
	Session *session.Session
}

func NewSshAdd(sess *session.Session) *SshAdd {
	return &SshAdd{Session: sess}
}

func (s *SshAdd) KeyCmd(keyPath string) *exec.Cmd {
	args := []string{
		keyPath,
	}
	env := []string{
		s.Session.AuthSockEnv(),
	}
	cmd := exec.Command(SshAddPath, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func (s *SshAdd) ListCmd() *exec.Cmd {
	env := []string{
		s.Session.AuthSockEnv(),
	}
	cmd := exec.Command(SshAddPath, "-l")
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func (s *SshAdd) AddKeys(keys []string) error {
	for _, k := range keys {
		app.Debugf("add key %s\n", k)
		args := []string{
			k,
		}
		env := []string{
			s.Session.AuthSockEnv(),
		}
		cmd := exec.Command(SshAddPath, args...)
		cmd.Env = append(os.Environ(), env...)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ssh-add: %s %v", string(output), err)
		}
		str := string(output)
		if str != "" && str != "\n" {
			log.InfoF("ssh-add: %s\n", output)
		}
	}

	if app.IsDebug {
		app.Debugf("list added keys\n")
		env := []string{
			s.Session.AuthSockEnv(),
		}
		cmd := exec.Command(SshAddPath, "-l")
		cmd.Env = append(os.Environ(), env...)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ssh-add -l: %v", err)
		}

		str := string(output)
		if str != "" && str != "\n" {
			log.InfoF("ssh-add -l: %s\n", output)
		}
	}

	return nil
}
