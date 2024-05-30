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

package frontend

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type Command struct {
	*process.Executor

	Session *session.Session

	Name string
	Args []string
	Env  []string

	SSHArgs []string

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
	c.SSHArgs = args
	return c
}

func (c *Command) OnCommandStart(fn func()) *Command {
	c.onCommandStart = fn
	return c
}

func (c *Command) Sudo() *Command {
	cmdLine := c.Name + " " + strings.Join(c.Args, " ")
	sudoCmdLine := fmt.Sprintf(
		`sudo -p SudoPassword -H -S -i bash -c 'echo SUDO-SUCCESS && %s'`,
		cmdLine,
	)

	var args []string
	args = append(args, c.SSHArgs...)
	args = append(args, []string{
		"-t", // allocate tty to auto kill remote process when ssh process is killed
		"-t", // need to force tty allocation because of stdin is pipe!
	}...)

	c.cmd = cmd.NewSSH(c.Session).
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
				log.DebugLn("Send become pass to cmd")
				_, _ = c.Executor.Stdin.Write([]byte(app.BecomePass + "\n"))
				passSent = true
			} else {
				// Second prompt is error!
				log.ErrorLn("Bad sudo password, exiting. TODO handle this correctly.")
				os.Exit(1)
			}
			return "reset"
		}
		if pattern == "SUDO-SUCCESS" {
			log.DebugLn("Got SUCCESS")
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
	c.cmd = cmd.NewSSH(c.Session).
		WithArgs(c.SSHArgs...).
		WithCommand(c.Name, c.Args...).Cmd()

	c.Executor = process.NewDefaultExecutor(c.cmd)
	return c
}

func (c *Command) Output() ([]byte, []byte, error) {
	if c.Session == nil {
		return nil, nil, fmt.Errorf("execute command %s: SSH client is undefined", c.Name)
	}

	c.cmd = cmd.NewSSH(c.Session).
		WithArgs(c.SSHArgs...).
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.Output()
	if err != nil {
		return output, nil, fmt.Errorf("execute command '%s': %w", c.Name, err)
	}
	return output, nil, nil
}

func (c *Command) CombinedOutput() ([]byte, error) {
	if c.Session == nil {
		return nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = cmd.NewSSH(c.Session).
		//	//WithArgs().
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("execute command '%s': %w", c.Name, err)
	}
	return output, nil
}

func (c *Command) WithTimeout(timeout time.Duration) *Command {
	c.Executor = c.Executor.WithTimeout(timeout)
	return c
}
