package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh/session"
)

const SSHAddPath = "ssh-add"

type SSHAdd struct {
	Session *session.Session
}

func NewSSHAdd(sess *session.Session) *SSHAdd {
	return &SSHAdd{Session: sess}
}

func (s *SSHAdd) KeyCmd(keyPath string) *exec.Cmd {
	args := []string{
		keyPath,
	}
	env := []string{
		s.Session.AuthSockEnv(),
	}
	cmd := exec.Command(SSHAddPath, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func (s *SSHAdd) ListCmd() *exec.Cmd {
	env := []string{
		s.Session.AuthSockEnv(),
	}
	cmd := exec.Command(SSHAddPath, "-l")
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func (s *SSHAdd) AddKeys(keys []string) error {
	for _, k := range keys {
		log.DebugF("add key %s\n", k)
		args := []string{
			k,
		}
		env := []string{
			s.Session.AuthSockEnv(),
		}
		cmd := exec.Command(SSHAddPath, args...)
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
		log.DebugLn("list added keys")
		env := []string{
			s.Session.AuthSockEnv(),
		}
		cmd := exec.Command(SSHAddPath, "-l")
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
