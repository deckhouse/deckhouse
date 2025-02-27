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
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"golang.org/x/crypto/ssh"
)

type SSHCommand struct {
	sshClient *ssh.Client
	Session   *ssh.Session

	Name string
	Args []string
	Env  []string

	SSHArgs []string

	onCommandStart func()
	stderrHandler  func(string)
	stdoutHandler  func(string)

	out bytes.Buffer
	err bytes.Buffer

	cmd string
}

func NewSSHCommand(client *ssh.Client, name string, arg ...string) *SSHCommand {
	args := make([]string, len(arg))
	copy(args, arg)
	cmd := name + " "
	for i := range args {
		if !strings.HasPrefix(args[i], `"`) &&
			!strings.HasSuffix(args[i], `"`) &&
			strings.Contains(args[i], " ") {
			args[i] = strconv.Quote(args[i])
			cmd = cmd + args[i] + " "
		}
	}
	session, _ := client.NewSession()

	return &SSHCommand{
		// Executor: process.NewDefaultExecutor(sess.Run(cmd)),
		sshClient: client,
		Session:   session,
		Name:      name,
		Args:      args,
		Env:       os.Environ(),
		cmd:       cmd,
	}
}

func (c *SSHCommand) WithSSHArgs(args ...string) {
	c.SSHArgs = args
}

func (c *SSHCommand) OnCommandStart(fn func()) {
	c.onCommandStart = fn
}

func (c *SSHCommand) Run() error {
	defer c.Session.Close()
	err := c.Session.Run(c.cmd)

	return err
}

func (c *SSHCommand) StderrBytes() []byte {
	return c.err.Bytes()
}

func (c *SSHCommand) StdoutBytes() []byte {
	return c.out.Bytes()
}

func (c *SSHCommand) Sudo() {
	cmdLine := c.Name + " " + strings.Join(c.Args, " ")
	sudoCmdLine := fmt.Sprintf(
		`sudo -p SudoPassword -H -S -i bash -c 'echo SUDO-SUCCESS && %s'`,
		cmdLine,
	)

	defer c.Session.Close()

	c.Session.Stdout = &c.out
	c.Session.Stderr = &c.err

	c.Session.Start(sudoCmdLine)

	stdin, _ := c.Session.StdinPipe()
	re := regexp.MustCompile(`SudoPassword`)
	re2 := regexp.MustCompile(`SUDO-SUCCESS`)
	passwordSent := false
	for {
		if len(c.out.Bytes()) > 0 {
			if re.Match(c.err.Bytes()) {
				if !passwordSent {
					becomePass := app.BecomePass
					stdin.Write([]byte(becomePass + "\n"))
					c.err.Reset()
					passwordSent = true
				} else {
					log.ErrorLn("Bad sudo password, exiting. TODO handle this correctly.")
					os.Exit(1)
				}
			}
		}
		if len(c.out.Bytes()) > 0 {
			if re2.Match(c.out.Bytes()) {
				break
			}
		}
	}

	stdin.Close()

	// Wait for the command to finish
	if err := c.Session.Wait(); err != nil {
		log.ErrorF("Command finish with an error %w", err)
		os.Exit(1)
	}
}

func (c *SSHCommand) WithStdoutHandler(handler func(string)) {
	c.stdoutHandler = handler
}

func (c *SSHCommand) WithStderrHandler(handler func(string)) {
	c.stderrHandler = handler
}

func (c *SSHCommand) Cmd() {
	defer c.Session.Close()
	c.Session.Stdout = &c.out
	c.Session.Stderr = &c.err
	c.Session.Run(c.cmd)
	c.Session.Wait()
}

func (c *SSHCommand) Output() ([]byte, []byte, error) {
	defer c.Session.Close()

	output, err := c.Session.Output(c.cmd)
	if err != nil {
		return output, nil, fmt.Errorf("execute command '%s': %w", c.Name, err)
	}
	return output, nil, nil
}

func (c *SSHCommand) CombinedOutput() ([]byte, error) {
	defer c.Session.Close()

	output, err := c.Session.CombinedOutput(c.cmd)
	if err != nil {
		return output, fmt.Errorf("execute command '%s': %w", c.Name, err)
	}
	return output, nil
}

func (c *SSHCommand) WithTimeout(timeout time.Duration) {
	// c.Executor = c.Executor.WithTimeout(timeout)
}

func (c *SSHCommand) WithEnv(env map[string]string) {
	c.Env = make([]string, 0, len(env))
	for k, v := range env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}
}
