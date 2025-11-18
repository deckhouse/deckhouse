// Copyright 2025 Flant JSC
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

package gossh

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	ssh_testing "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh/testing"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/stretchr/testify/require"
)

func TestCommandOutput(t *testing.T) {
	testName := "TestCommandOutput"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container without password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20027, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20027"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
	})

	t.Run("Get command Output", func(t *testing.T) {
		cases := []struct {
			title             string
			command           string
			args              []string
			expectedOutput    string
			expectedErrOutput string
			timeout           time.Duration
			prepareFunc       func(c *SSHCommand) error
			wantErr           bool
			err               string
		}{
			{
				title:          "Just echo, success",
				command:        "echo",
				args:           []string{"\"test output\""},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
			{
				title:          "With context",
				command:        "while true; do echo \"test\"; sleep 5; done",
				args:           []string{},
				expectedOutput: "test\ntest\n",
				timeout:        7 * time.Second,
				wantErr:        false,
			},
			{
				title:             "Command return error",
				command:           "cat",
				args:              []string{"\"/etc/sudoers\""},
				wantErr:           true,
				err:               "Process exited with status 1",
				expectedErrOutput: "cat: /etc/sudoers: Permission denied\n",
			},
			{
				title:   "With opened stdout pipe",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					return c.Run(context.Background())
				},
				wantErr: true,
				err:     "open stdout pipe",
			},
			{
				title:   "With opened stderr pipe",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					buf := new(bytes.Buffer)
					c.session.Stderr = buf
					return nil
				},
				wantErr: true,
				err:     "open stderr pipe",
			},
			{
				title:   "With nil session",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					err := c.session.Close()
					c.session = nil
					return err
				},
				wantErr: true,
				err:     "ssh session not started",
			},
			{
				title:   "With defined buffers",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					c.out = new(bytes.Buffer)
					c.err = new(bytes.Buffer)
					return nil
				},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				cmd := NewSSHCommand(sshClient, c.command, c.args...)
				ctx := context.Background()
				var emptyDuration time.Duration
				var cancel context.CancelFunc
				if c.timeout != emptyDuration {
					ctx, cancel = context.WithDeadline(ctx, time.Now().Add(c.timeout))
				}
				if cancel != nil {
					defer cancel()
				}
				if c.prepareFunc != nil {
					err = c.prepareFunc(cmd)
					require.NoError(t, err)
				}
				out, errBytes, err := cmd.Output(ctx)
				if !c.wantErr {
					require.NoError(t, err)
					require.Equal(t, c.expectedOutput, string(out))
				} else {
					require.Error(t, err)
					require.Equal(t, c.expectedErrOutput, string(errBytes))
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})
}

func TestCommandCombinedOutput(t *testing.T) {
	testName := "TestCommandCombinedOutput"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container without password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20028, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20028"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
	})

	t.Run("Get command CombinedOutput", func(t *testing.T) {
		cases := []struct {
			title             string
			command           string
			args              []string
			expectedOutput    string
			expectedErrOutput string
			timeout           time.Duration
			prepareFunc       func(c *SSHCommand) error
			wantErr           bool
			err               string
		}{
			{
				title:          "Just echo, success",
				command:        "echo",
				args:           []string{"\"test output\""},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
			{
				title:          "With context",
				command:        "while true; do echo \"test\"; sleep 5; done",
				args:           []string{},
				expectedOutput: "test\ntest\n",
				timeout:        7 * time.Second,
				wantErr:        false,
			},
			{
				title:             "Command return error",
				command:           "cat",
				args:              []string{"\"/etc/sudoers\""},
				wantErr:           true,
				err:               "Process exited with status 1",
				expectedErrOutput: "cat: /etc/sudoers: Permission denied\n",
			},
			{
				title:   "With opened stdout pipe",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					return c.Run(context.Background())
				},
				wantErr: true,
				err:     "open stdout pipe",
			},
			{
				title:   "With opened stderr pipe",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					buf := new(bytes.Buffer)
					c.session.Stderr = buf
					return nil
				},
				wantErr: true,
				err:     "open stderr pipe",
			},
			{
				title:   "With nil session",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					err := c.session.Close()
					c.session = nil
					return err
				},
				wantErr: true,
				err:     "ssh session not started",
			},
			{
				title:   "With defined buffers",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					c.out = new(bytes.Buffer)
					c.err = new(bytes.Buffer)
					return nil
				},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				cmd := NewSSHCommand(sshClient, c.command, c.args...)
				ctx := context.Background()
				var emptyDuration time.Duration
				var cancel context.CancelFunc
				if c.timeout != emptyDuration {
					ctx, cancel = context.WithDeadline(ctx, time.Now().Add(c.timeout))
				}
				if cancel != nil {
					defer cancel()
				}
				if c.prepareFunc != nil {
					err = c.prepareFunc(cmd)
					require.NoError(t, err)
				}
				combined, err := cmd.CombinedOutput(ctx)
				if !c.wantErr {
					require.NoError(t, err)
					require.Equal(t, c.expectedOutput, string(combined))
				} else {
					require.Error(t, err)
					require.Equal(t, c.expectedErrOutput, string(combined))
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})
}

func TestCommandRun(t *testing.T) {
	testName := "TestCommandRun"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container without password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20028, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20028"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
	})

	// evns test
	envs := make(map[string]string)
	envs["TEST_ENV"] = "test"

	t.Run("Run a command", func(t *testing.T) {
		cases := []struct {
			title             string
			command           string
			args              []string
			expectedOutput    string
			expectedErrOutput string
			timeout           time.Duration
			prepareFunc       func(c *SSHCommand) error
			envs              map[string]string
			wantErr           bool
			err               string
		}{
			{
				title:          "Just echo, success",
				command:        "echo",
				args:           []string{"\"test output\""},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
			{
				title:          "Just echo, with envs, success",
				command:        "echo",
				args:           []string{"\"test output\""},
				expectedOutput: "test output\n",
				envs:           envs,
				wantErr:        false,
			},
			{
				title:          "With context",
				command:        "while true; do echo \"test\"; sleep 5; done",
				args:           []string{},
				expectedOutput: "test\ntest\n",
				timeout:        7 * time.Second,
				wantErr:        false,
			},
			{
				title:             "Command return error",
				command:           "cat",
				args:              []string{"\"/etc/sudoers\""},
				wantErr:           true,
				err:               "Process exited with status 1",
				expectedErrOutput: "cat: /etc/sudoers: Permission denied\n",
			},
			{
				title:   "With opened stdout pipe",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					return c.Run(context.Background())
				},
				wantErr: true,
				err:     "ssh: session already started",
			},
			{
				title:   "With nil session",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					err := c.session.Close()
					c.session = nil
					return err
				},
				wantErr: true,
				err:     "ssh session not started",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				cmd := NewSSHCommand(sshClient, c.command, c.args...)
				ctx := context.Background()
				var emptyDuration time.Duration
				var cancel context.CancelFunc
				if c.timeout != emptyDuration {
					ctx, cancel = context.WithDeadline(ctx, time.Now().Add(c.timeout))
				}
				if cancel != nil {
					defer cancel()
				}
				if c.prepareFunc != nil {
					err = c.prepareFunc(cmd)
					require.NoError(t, err)
				}
				if len(c.envs) > 0 {
					cmd.WithEnv(c.envs)
				}

				err = cmd.Run(ctx)
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}

				// second run for context after deadline exceeded
				if c.timeout != emptyDuration {
					cmd2 := NewSSHCommand(sshClient, c.command, c.args...)
					if c.prepareFunc != nil {
						err = c.prepareFunc(cmd2)
						require.NoError(t, err)
					}
					if len(c.envs) > 0 {
						cmd2.WithEnv(c.envs)
					}
					err = cmd2.Run(ctx)
					// command should fail to run
					require.Error(t, err)
					require.Contains(t, err.Error(), "context deadline exceeded")

				}
			})
		}
	})
}

func TestCommandStart(t *testing.T) {
	testName := "TestCommandStart"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container without password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20029, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20029"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
	})

	t.Run("Start and stop a command", func(t *testing.T) {
		cases := []struct {
			title             string
			command           string
			args              []string
			expectedOutput    string
			expectedErrOutput string
			timeout           time.Duration
			prepareFunc       func(c *SSHCommand) error
			wantErr           bool
			err               string
		}{
			{
				title:          "Just echo, success",
				command:        "echo",
				args:           []string{"\"test output\""},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
			{
				title:          "With context",
				command:        "while true; do echo \"test\"; sleep 5; done",
				args:           []string{},
				expectedOutput: "test\ntest\n",
				timeout:        7 * time.Second,
				wantErr:        false,
			},
			{
				title:             "Command return error",
				command:           "cat",
				args:              []string{"\"/etc/sudoers\""},
				wantErr:           true,
				err:               "Process exited with status 1",
				expectedErrOutput: "cat: /etc/sudoers: Permission denied\n",
			},
			{
				title:   "With opened stdout pipe",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					return c.Run(context.Background())
				},
				wantErr: true,
				err:     "ssh: session already started",
			},
			{
				title:   "With nil session",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					err := c.session.Close()
					c.session = nil
					return err
				},
				wantErr: true,
				err:     "ssh session not started",
			},
			{
				title:   "waitHandler",
				command: "echo",
				args:    []string{"\"test output\""},
				prepareFunc: func(c *SSHCommand) error {
					c.WithWaitHandler(func(err error) {
						if err != nil {
							log.ErrorF("SSH-agent process exited, now stop. Wait error: %v\n", err)
							return
						}
						log.InfoF("SSH-agent process exited, now stop.\n")
					})
					return nil
				},
				expectedOutput: "test output\n",
				wantErr:        false,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				cmd := NewSSHCommand(sshClient, c.command, c.args...)
				ctx := context.Background()
				var emptyDuration time.Duration
				if c.timeout != emptyDuration {
					cmd.WithTimeout(c.timeout)
				}
				if c.prepareFunc != nil {
					err = c.prepareFunc(cmd)
					require.NoError(t, err)
				}
				cmd.Cmd(ctx)
				err = cmd.Start()
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
				cmd.Stop()
			})
		}
	})
}

func TestCommandSudoRun(t *testing.T) {
	testName := "TestCommandSudoRun"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container without password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20030, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	// starting openssh container with password auth
	containerWithPass := ssh_testing.NewSSHContainer(publicKey, "", "VeryStrongPasswordWhatCannotBeGuessed", "user", 20031, true)
	err = containerWithPass.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20030"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)
	settings = session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20031",
		BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed",
	})
	sshClient2 := NewClient(settings, make([]session.AgentPrivateKey, 0, 1))
	err = sshClient2.Start()
	// expecting no error on client start
	require.NoError(t, err)

	// client with wrong sudo password
	settings = session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20031",
		BecomePass:     "WrongPassword",
	})
	sshClient3 := NewClient(settings, keys)
	err = sshClient3.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		sshClient2.Stop()
		sshClient3.Stop()
		container.Stop()
		containerWithPass.Stop()
		os.Remove(path)
	})

	t.Run("Run a command", func(t *testing.T) {
		cases := []struct {
			title       string
			sshClient   *Client
			command     string
			args        []string
			timeout     time.Duration
			prepareFunc func(c *SSHCommand) error
			wantErr     bool
			err         string
			errorOutput string
		}{
			{
				title:     "Just echo, success",
				sshClient: sshClient,
				command:   "echo",
				args:      []string{"\"test output\""},
				wantErr:   false,
			},
			{
				title:     "Just echo, success, with password",
				sshClient: sshClient2,
				command:   "echo",
				args:      []string{"\"test output\""},
				wantErr:   false,
			},
			{
				title:       "Just echo, failure, with wrong password",
				sshClient:   sshClient3,
				command:     "echo",
				args:        []string{"\"test output\""},
				wantErr:     true,
				err:         "Process exited with status 1",
				errorOutput: "SudoPasswordSorry, try again.\nSudoPasswordSorry, try again.\nSudoPasswordsudo: 3 incorrect password attempts\n",
			},
			{
				title:     "With context",
				sshClient: sshClient,
				command:   "while true; do echo \"test\"; sleep 5; done",
				args:      []string{},
				timeout:   7 * time.Second,
				wantErr:   false,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				cmd := NewSSHCommand(c.sshClient, c.command, c.args...).CaptureStderr(nil)
				ctx := context.Background()
				var emptyDuration time.Duration
				var cancel context.CancelFunc
				if c.timeout != emptyDuration {
					ctx, cancel = context.WithDeadline(ctx, time.Now().Add(c.timeout))
				}
				if cancel != nil {
					defer cancel()
				}
				if c.prepareFunc != nil {
					err = c.prepareFunc(cmd)
					require.NoError(t, err)
				}
				cmd.Sudo(ctx)
				err = cmd.Run(ctx)
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
					errBytes := cmd.StderrBytes()

					require.Contains(t, string(errBytes), c.errorOutput)
				}
			})
		}
	})
}
