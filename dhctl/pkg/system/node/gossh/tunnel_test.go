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
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	ssh_testing "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh/testing"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/stretchr/testify/require"
)

func TestTunnel(t *testing.T) {
	testName := "TestTunnel"

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
	err = container.WriteConfig()
	if err != nil {
		// cannot start test w/o container
		return
	}
	err = container.Start()
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
	sshClient := NewClient(context.Background(), settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		container.RemoveConfig()
	})

	t.Run("Tunnel to container", func(t *testing.T) {
		cases := []struct {
			title   string
			address string
			wantErr bool
			err     string
		}{
			{
				title:   "Tunnel, success",
				address: "127.0.0.1:2222:127.0.0.1:22050",
				wantErr: false,
			},
			{
				title:   "Invalid address",
				address: "22050:127.0.0.1:2222",
				wantErr: true,
				err:     "invalid address must be 'remote_bind:remote_port:local_bind:local_port'",
			},
			{
				title:   "Invalid local bind",
				address: "127.0.0.1:2222:127.0.0.1:22",
				wantErr: true,
				err:     "failed to listen local on 127.0.0.1:22",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				tun := NewTunnel(sshClient, c.address)
				err = tun.Up()
				if !c.wantErr {
					require.NoError(t, err)
					// try to up again: expectiong error
					err = tun.Up()
					require.Error(t, err)
					require.Equal(t, err.Error(), "already up")
					newSettings := session.NewSession(session.Input{
						AvailableHosts: []session.Host{{Host: "127.0.0.1", Name: "localhost"}},
						User:           "user",
						Port:           "22050"})
					newSSHClient := NewClient(context.Background(), newSettings, keys)
					err = newSSHClient.Start()
					require.NoError(t, err)
					tun.Stop()
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
				// call stop on closed tun should not cause any problems
				tun.Stop()
			})
		}
	})
}

func TestHealthMonitor(t *testing.T) {
	testName := "TestTunnelHealthMonitor"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container without password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20031, true)
	err = container.WriteConfig()
	if err != nil {
		// cannot start test w/o container
		return
	}
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20031"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(context.Background(), settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		container.RemoveConfig()
	})
	t.Run("Dial to unreacheble host", func(t *testing.T) {
		tun := NewTunnel(sshClient, "100.200.200.300:80:127.0.0.1:8080")
		err = tun.Up()
		require.NoError(t, err)

		// starting HealthMonitor
		errChan := make(chan error, 10)
		go tun.HealthMonitor(errChan)

		req, err := http.NewRequest("GET", "http://127.0.0.1:8080", nil)
		require.NoError(t, err)

		client := &http.Client{}
		client.Timeout = 2 * time.Second
		_, err = client.Do(req)
		require.Error(t, err)

		msg := <-errChan
		require.Contains(t, msg.Error(), "Cannot dial to 100.200.200.300:80")

		tun.Stop()
	})
	t.Run("String func test", func(t *testing.T) {
		cases := []struct {
			title    string
			address  string
			expected string
		}{
			{
				title:    "Normal address",
				address:  "127.0.0.1:2222:127.0.0.1:22050",
				expected: "L:127.0.0.1:2222:127.0.0.1:22050",
			},
			{
				title:    "Invalid address",
				address:  "22050:127.0.0.1:2222",
				expected: "L:22050:127.0.0.1:2222",
			},
			{
				title:    "Remote FQDN",
				address:  "www.example.com:8080:127.0.0.1:8080",
				expected: "L:www.example.com:8080:127.0.0.1:8080",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				tun := NewTunnel(sshClient, c.address)
				require.Equal(t, c.expected, tun.String())

			})
		}
	})
}
