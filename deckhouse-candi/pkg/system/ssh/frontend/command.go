package frontend

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/process"
	"flant/deckhouse-candi/pkg/system/ssh/cmd"
	"flant/deckhouse-candi/pkg/system/ssh/session"
)

type Command struct {
	*process.Executor

	Session *session.Session

	Name string
	Args []string
	Env  []string

	SshArgs []string

	onCommandStart func()

	cmd *exec.Cmd
}

func NewCommand(sess *session.Session, name string, arg ...string) *Command {
	return &Command{
		Session: sess,
		Name:    name,
		Args:    arg,
	}
}

func (c *Command) WithSSHArgs(args ...string) *Command {
	c.SshArgs = args
	return c
}

func (c *Command) OnCommandStart(fn func()) *Command {
	c.onCommandStart = fn
	return c
}

func (c *Command) Sudo() *Command {
	cmdLine := c.Name + " " + strings.Join(c.Args, " ")
	sudoCmdLine := fmt.Sprintf(`sudo -p SudoPassword -H -S -i bash -c 'echo SUDO-SUCCESS && %s'`, cmdLine)

	args := append(c.SshArgs, []string{
		"-t", // allocate tty to auto kill remote process when ssh process is killed
		"-t", // need to force tty allocation because of stdin is pipe!
	}...)

	c.cmd = cmd.NewSsh(c.Session).
		WithArgs(args...).
		WithCommand(sudoCmdLine).Cmd()

	c.Executor = process.NewDefaultExecutor(c.cmd)

	c.WithMatchers(
		process.NewByteSequenceMatcher("SudoPassword"),
		process.NewByteSequenceMatcher("SUDO-SUCCESS").WaitNonMatched(),
	)
	c.OpenStdinPipe()

	passSent := false
	c.WithMatchHandler(func(pattern string) string {
		if pattern == "SudoPassword" {
			if !passSent {
				// send pass through stdin
				app.Debugf("Send become pass to cmd\n")
				c.Executor.Stdin.Write([]byte(app.BecomePass + "\n"))
				passSent = true
			} else {
				// Second prompt is error!
				log.ErrorLn("Bad sudo password, exiting. TODO handle this correctly.")
				os.Exit(1)
			}
			return "reset"
		}
		if pattern == "SUDO-SUCCESS" {
			app.Debugf("Got SUCCESS\n")
			if c.onCommandStart != nil {
				c.onCommandStart()
			}
			return "done"
		}
		return ""
	})
	return c
}

func (c *Command) Cmd() *Command {
	c.cmd = cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...).Cmd()

	c.Executor = process.NewDefaultExecutor(c.cmd)
	return c
}

func (c *Command) Output() ([]byte, []byte, error) {
	if c.Session == nil {
		return nil, nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.Output()
	if err != nil {
		return output, nil, fmt.Errorf("execute command '%s': %v", c.Name, err)
	}
	return output, nil, nil
}

func (c *Command) CombinedOutput() ([]byte, error) {
	if c.Session == nil {
		return nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = cmd.NewSsh(c.Session).
		//	//WithArgs().
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		//fmt.Printf("%s: %s\n", c.Name, output)
		return output, fmt.Errorf("execute command '%s': %v", c.Name, err)
	}
	return output, nil
}
