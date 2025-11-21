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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	ssh_testing "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh/testing"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/stretchr/testify/require"
)

func TestReverseTunnel(t *testing.T) {
	testName := "TestReverseTunnel"

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
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	go func() {
		err = ssh_testing.StartWebServer(":8088")
		require.NoError(t, err)
	}()

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		container.RemoveConfig()
	})
	// we don't have /opt/deckhouse in the container, so we should create it before start any UploadScript with sudo
	err = container.CreateDeckhouseDirs()
	require.NoError(t, err)

	t.Run("Reverse tunnel from container to host", func(t *testing.T) {
		cases := []struct {
			title       string
			address     string
			wantErr     bool
			err         string
			errFromChan string
		}{
			{
				title:   "Tunnel, success",
				address: "127.0.0.1:8080:127.0.0.1:8088",
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
				err:     "failed to listen remote on 127.0.0.1:2222",
			},
			{
				title:       "Wrong local bind",
				address:     "127.0.0.1:8080:127.0.0.1:8087",
				wantErr:     false,
				errFromChan: "Cannot dial to 127.0.0.1:8087",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				tun := NewReverseTunnel(sshClient, c.address)
				err = tun.Up()
				if !c.wantErr {
					require.NoError(t, err)
					// try to up again: expectiong error
					err = tun.Up()
					require.Error(t, err)
					require.Equal(t, err.Error(), "already up")
					// try to get a response from local web server
					cmd := NewSSHCommand(sshClient, "curl", "-s", "http://127.0.0.1:8080")
					cmd.WithTimeout(2 * time.Second)
					out, err := cmd.CombinedOutput(context.Background())
					require.NoError(t, err)
					if len(c.errFromChan) == 0 {
						require.Equal(t, "This is a simple web server response", string(out))
					} else {
						errMsg := <-tun.errorCh
						require.Contains(t, errMsg.err.Error(), c.errFromChan)
					}
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

	t.Run("String func test", func(t *testing.T) {
		cases := []struct {
			title    string
			address  string
			expected string
		}{
			{
				title:    "Normal address",
				address:  "127.0.0.1:2222:127.0.0.1:22050",
				expected: "R:127.0.0.1:2222:127.0.0.1:22050",
			},
			{
				title:    "Invalid address",
				address:  "22050:127.0.0.1:2222",
				expected: "R:22050:127.0.0.1:2222",
			},
			{
				title:    "Remote FQDN",
				address:  "www.example.com:8080:127.0.0.1:8080",
				expected: "R:www.example.com:8080:127.0.0.1:8080",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				tun := NewReverseTunnel(sshClient, c.address)
				require.Equal(t, c.expected, tun.String())

			})
		}
	})

	t.Run("HealthMonitor test", func(t *testing.T) {
		tun := NewReverseTunnel(sshClient, "127.0.0.1:8080:127.0.0.1:8088")
		err := tun.Up()
		require.NoError(t, err)
		// creating direactory to upload
		testDir := filepath.Join(os.TempDir(), "dhctltests", "script")
		err = os.MkdirAll(testDir, 0755)
		require.NoError(t, err)

		testFile, err := os.Create(filepath.Join(testDir, "test.sh"))
		require.NoError(t, err)
		script := `#!/bin/bash
URL="http://127.0.0.1:8080"

curl -s $URL > /dev/null
exit $?
`
		testFile.WriteString(script)
		testFile.Chmod(0o755)
		checker := ssh.NewRunScriptReverseTunnelChecker(sshClient, testFile.Name())
		killer := ssh.EmptyReverseTunnelKiller{}

		err = retry.NewSilentLoop("check tunnel", 30, 2*time.Second).Run(func() error {
			out, err := checker.CheckTunnel(context.Background())
			if err != nil {
				log.InfoF("failed to check tunnel: %s %v", out, err)
				return err
			}
			return nil
		})
		require.NoError(t, err)

		tun.StartHealthMonitor(context.Background(), checker, killer)
		time.Sleep(5 * time.Second)
		err = container.Stop()
		require.NoError(t, err)
		time.Sleep(5 * time.Second)
		err = container.WithNetwork("")
		require.NoError(t, err)
		err = container.Start()
		require.NoError(t, err)
		err = container.CreateDeckhouseDirs()
		require.NoError(t, err)

		time.Sleep(30 * time.Second)
		err = retry.NewSilentLoop("check tunnel", 10, 5*time.Second).Run(func() error {
			out, err := checker.CheckTunnel(context.Background())
			if err != nil {
				log.InfoF("failed to check tunnel: %s %v", out, err)
				return err
			}
			return nil
		})
		require.NoError(t, err)

		// disconnect/connect case
		err = container.Disconnect()
		require.NoError(t, err)
		time.Sleep(5 * time.Second)
		err = container.Connect()
		require.NoError(t, err)
		time.Sleep(30 * time.Second)
		err = retry.NewSilentLoop("check tunnel", 10, 5*time.Second).Run(func() error {
			out, err := checker.CheckTunnel(context.Background())
			if err != nil {
				log.InfoF("failed to check tunnel: %s %v", out, err)
				return err
			}
			return nil
		})
		require.NoError(t, err)
		tun.Stop()
	})
}
