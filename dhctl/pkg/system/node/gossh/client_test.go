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
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	ssh_testing "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh/testing"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/stretchr/testify/require"
)

func TestOnlyPreparePrivateKeys(t *testing.T) {
	// genetaring ssh keys
	path, _, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}
	tmpFile, _ := os.CreateTemp("/tmp", "wrong-key")
	_, err = tmpFile.WriteString("Hello world")
	if err != nil {
		return
	}
	keyWithPass, _, err := ssh_testing.GenerateKeys("password")
	if err != nil {
		return
	}

	t.Cleanup(func() {
		os.Remove(path)
		os.Remove(tmpFile.Name())
		os.Remove(keyWithPass)
	})
	t.Run("OnlyPrepareKeys cases", func(t *testing.T) {
		cases := []struct {
			title    string
			settings session.Session
			keys     []session.AgentPrivateKey
			wantErr  bool
			err      string
		}{
			{
				title: "No keys",
				settings: *session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022",
					BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed"}),
				keys:    make([]session.AgentPrivateKey, 0, 1),
				wantErr: false,
			},
			{
				title: "Key auth, no password",
				settings: *session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: path}},
				wantErr: false,
			},
			{
				title: "Key auth, no password, noexistent key",
				settings: *session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: "/tmp/noexistent-key"}},
				wantErr: true,
				err:     "open /tmp/noexistent-key: no such file or directory",
			},
			{
				title: "Key auth, no password, wrong key",
				settings: *session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: tmpFile.Name()}},
				wantErr: true,
				err:     "ssh: no key found",
			},
			{
				title: "Key auth, with passphrase",
				settings: *session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: keyWithPass, Passphrase: "password"}},
				wantErr: false,
			},
			{
				title: "Key auth, with wrong passphrase",
				settings: *session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: keyWithPass, Passphrase: "wrongpassword"}},
				wantErr: true,
				err:     "x509: decryption password incorrect",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				var sshClient *Client
				if c.settings.BecomePass != "" {
					app.BecomePass = c.settings.BecomePass
				}
				sshClient = NewClient(&c.settings, c.keys)
				err := sshClient.OnlyPreparePrivateKeys()
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}

				// double run
				err = sshClient.OnlyPreparePrivateKeys()
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}

			})
		}

	})
}

func TestClientStart(t *testing.T) {
	testName := "TestClientStart"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "VeryStrongPasswordWhatCannotBeGuessed", "user", 20022, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	// starting openssh container (bastion) with key auth and AllowTcpForwarding yes in config
	bastion := ssh_testing.NewSSHContainer(publicKey, "", "VeryStrongPasswordWhatCannotBeGuessed", "bastionuser", 20023, true)
	err = bastion.WriteConfig()
	if err != nil {
		// cannot start test w/o config file
		return
	}
	err = bastion.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}
	auth_sock := os.Getenv("SSH_AUTH_SOCK")
	if auth_sock != "" {
		// add key to agent
		cmd := exec.Command("ssh-add", path)
		cmd.Run()
	}

	t.Cleanup(func() {
		container.Stop()
		bastion.Stop()
		if auth_sock != "" {
			cmd := exec.Command("ssh-add", "-d", path)
			cmd.Run()
		}
		os.Remove(path)
		bastion.RemoveConfig()
	})

	t.Run("Start ssh client against single host", func(t *testing.T) {
		cases := []struct {
			title     string
			settings  *session.Session
			keys      []session.AgentPrivateKey
			wantErr   bool
			err       string
			auth_sock string
		}{
			{
				title: "Password auth, no keys",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022",
					BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed"}),
				keys:    make([]session.AgentPrivateKey, 0, 1),
				wantErr: false,
			},
			{
				title: "Key auth, no password",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: path}},
				wantErr: false,
			},
			{
				title: "SSH_AUTH_SOCK auth",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:      []session.AgentPrivateKey{{Key: path}},
				wantErr:   false,
				auth_sock: auth_sock,
			},
			{
				title: "SSH_AUTH_SOCK auth, wrong socket",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:      make([]session.AgentPrivateKey, 0, 1),
				wantErr:   true,
				err:       "Failed to open SSH_AUTH_SOCK",
				auth_sock: "/run/nonexistent",
			},
			{
				title: "Key auth, no password, wrong key",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:    []session.AgentPrivateKey{{Key: "/tmp/noexistent-key"}},
				wantErr: true,
			},
			{
				title:    "No session",
				settings: nil,
				keys:     []session.AgentPrivateKey{{Key: "/tmp/noexistent-key"}},
				wantErr:  true,
				err:      "possible bug in ssh client: session should be created before start",
			},
			{
				title: "No auth",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20022"}),
				keys:      make([]session.AgentPrivateKey, 0, 1),
				wantErr:   true,
				err:       "one of SSH keys, SSH_AUTH_SOCK environment variable or become password should be not empty",
				auth_sock: "",
			},
			{
				title: "Wrong port",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
					User:           "user",
					Port:           "20021"}),
				keys:      []session.AgentPrivateKey{{Key: path}},
				wantErr:   true,
				err:       "Failed to connect to master host",
				auth_sock: "",
			},
			{
				title: "With bastion, key auth",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: container.IP, Name: container.IP}},
					User:           "user",
					Port:           "2222",
					BastionHost:    "localhost",
					BastionPort:    "20023",
					BastionUser:    bastion.Username,
				}),
				keys:      []session.AgentPrivateKey{{Key: path}},
				wantErr:   false,
				auth_sock: "",
			},
			{
				title: "With bastion, password auth",
				settings: session.NewSession(session.Input{
					AvailableHosts:  []session.Host{{Host: container.IP, Name: container.IP}},
					User:            "user",
					Port:            "2222",
					BecomePass:      "VeryStrongPasswordWhatCannotBeGuessed",
					BastionHost:     "localhost",
					BastionPort:     "20023",
					BastionUser:     bastion.Username,
					BastionPassword: "VeryStrongPasswordWhatCannotBeGuessed",
				}),
				keys:      make([]session.AgentPrivateKey, 0, 1),
				wantErr:   false,
				auth_sock: "",
			},
			{
				title: "With bastion, no auth",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: container.IP, Name: container.IP}},
					User:           "user",
					Port:           "2222",
					BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed",
					BastionHost:    "localhost",
					BastionPort:    "20023",
					BastionUser:    bastion.Username,
				}),
				keys:      make([]session.AgentPrivateKey, 0, 1),
				wantErr:   true,
				err:       "No credentials present to connect to bastion host",
				auth_sock: "",
			},
			{
				title: "With bastion, SSH_AUTH_SOCK auth",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: container.IP, Name: container.IP}},
					User:           "user",
					Port:           "2222",
					BastionHost:    "localhost",
					BastionPort:    "20023",
					BastionUser:    bastion.Username,
				}),
				keys:      []session.AgentPrivateKey{{Key: path}},
				wantErr:   false,
				auth_sock: auth_sock,
			},
			{
				title: "With bastion, key auth, wrong target host",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: container.IP, Name: container.IP}},
					User:           "user",
					Port:           "20022",
					BastionHost:    "localhost",
					BastionPort:    "20023",
					BastionUser:    bastion.Username,
				}),
				keys:      []session.AgentPrivateKey{{Key: path}},
				wantErr:   true,
				err:       "Failed to connect to target host through bastion host",
				auth_sock: "",
			},
			{
				title: "With bastion, key auth, wrong bastion port",
				settings: session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: container.IP, Name: container.IP}},
					User:           "user",
					Port:           "2222",
					BastionHost:    "localhost",
					BastionPort:    "20021",
					BastionUser:    bastion.Username,
				}),
				keys:      []session.AgentPrivateKey{{Key: path}},
				wantErr:   true,
				err:       "Could not connect to bastion host",
				auth_sock: "",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				os.Setenv("SSH_AUTH_SOCK", c.auth_sock)
				app.BecomePass = ""
				app.SSHBastionPass = ""
				var sshClient *Client
				if c.settings != nil {
					if c.settings.BecomePass != "" {
						app.BecomePass = c.settings.BecomePass
					}
					if c.settings.BastionPassword != "" {
						app.SSHBastionPass = c.settings.BastionPassword
					}
				}

				fmt.Println("starting ssh client")
				sshClient = NewClient(c.settings, c.keys)
				err = sshClient.Start()
				if !c.wantErr {
					require.NoError(t, err)
					fmt.Println("client started successfully")
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
				fmt.Println("stopping ssh client")
				sshClient.Stop()
				fmt.Println("ssh client has been stoped")

			})
		}

	})
}

func TestClientKeepalive(t *testing.T) {
	testName := "TestClientKeepalive"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "VeryStrongPasswordWhatCannotBeGuessed", "user", 20022, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	t.Cleanup(func() {
		container.Stop()
		os.Remove(path)
	})
	os.Setenv("SSH_AUTH_SOCK", "")

	t.Run("keepalive test", func(t *testing.T) {
		settings := session.NewSession(session.Input{
			AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
			User:           "user",
			Port:           "20022"})
		keys := []session.AgentPrivateKey{{Key: path}}
		sshClient := NewClient(settings, keys)
		err := sshClient.Start()
		// expecting no error on client start
		require.NoError(t, err)
		// test case: stopping container for a while, waiting for client recreation, creating new session, expecting no error
		time.Sleep(2 * time.Second)
		container.Stop()
		time.Sleep(5 * time.Second)
		container.Start()
		time.Sleep(30 * time.Second)
		sess, err := sshClient.GetClient().NewSession()
		require.NoError(t, err)
		sshClient.RegisterSession(sess)
		sshClient.Stop()
	})
}

func TestClientLoop(t *testing.T) {
	t.Run("Loop", func(t *testing.T) {
		settings := session.NewSession(session.Input{
			AvailableHosts: []session.Host{{Host: "127.0.0.1", Name: "localhost"}, {Host: "127.0.0.2"}},
			User:           "user",
			Port:           "20022",
			BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed"})
		keys := make([]session.AgentPrivateKey, 0, 1)
		sshClient := NewClient(settings, keys)

		err := sshClient.Loop(func(s node.SSHClient) error {
			keys := s.PrivateKeys()
			if len(keys) == 0 {
				return fmt.Errorf("keys are empty")
			}
			return nil
		})
		require.Error(t, err)
		err = sshClient.Loop(func(s node.SSHClient) error {
			keys := s.PrivateKeys()
			if len(keys) == 0 {
				return nil
			}
			return fmt.Errorf("keys are not empty")
		})
		require.NoError(t, err)
	})
}

func TestClientSettings(t *testing.T) {
	t.Run("settings", func(t *testing.T) {
		settings := session.NewSession(session.Input{
			AvailableHosts: []session.Host{{Host: "127.0.0.1", Name: "localhost"}, {Host: "127.0.0.2"}},
			User:           "user",
			Port:           "20022",
			BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed"})
		keys := make([]session.AgentPrivateKey, 0, 1)
		sshClient := NewClient(settings, keys)
		s := sshClient.Session()
		require.Equal(t, settings, s)
	})
}

func TestClientLive(t *testing.T) {
	t.Run("settings", func(t *testing.T) {
		settings := session.NewSession(session.Input{
			AvailableHosts: []session.Host{{Host: "127.0.0.1", Name: "localhost"}, {Host: "127.0.0.2"}},
			User:           "user",
			Port:           "20022",
			BecomePass:     "VeryStrongPasswordWhatCannotBeGuessed"})
		keys := make([]session.AgentPrivateKey, 0, 1)
		sshClient := NewClient(settings, keys)
		live := sshClient.Live()
		require.Equal(t, false, live)
	})
}
