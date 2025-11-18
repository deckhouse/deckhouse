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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	ssh_testing "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh/testing"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/stretchr/testify/require"
)

func TestUploadScriptExecute(t *testing.T) {
	testName := "TestUploadScriptExecute"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20025, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	// creating direactory to upload
	testDir := filepath.Join(os.TempDir(), "dhctltests", "script")
	err = os.MkdirAll(testDir, 0755)
	if err != nil {
		// cannot start test w/o files to upload
		return
	}

	testFile, err := os.Create(filepath.Join(testDir, "test.sh"))
	if err != nil {
		// cannot start test w/o script to upload
		return
	}
	script := `#!/bin/bash

if [[ $# -eq 0 ]]; then
  echo "Error: No arguments provided."
  exit 1
elif [[ $# -gt 1 ]]; then
  echo "Usage: $0 <arg1>"
  exit 1
else
  echo "provided: $1"
fi
`
	testFile.WriteString(script)
	testFile.Chmod(0o755)

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20025"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		os.RemoveAll(testDir)
	})
	// we don't have /opt/deckhouse in the container, so we should create it before start any UploadScript with sudo
	cmd := sshClient.Command("mkdir", "-p", app.DeckhouseNodeTmpPath)
	cmd.Sudo(context.Background())
	err = cmd.Run(context.Background())
	require.NoError(t, err)
	cmd = sshClient.Command("chmod", "777", app.DeckhouseNodeTmpPath)
	cmd.Sudo(context.Background())
	err = cmd.Run(context.Background())
	require.NoError(t, err)

	// evns test
	envs := make(map[string]string)
	envs["TEST_ENV"] = "test"

	t.Run("Upload script to container via existing ssh client", func(t *testing.T) {
		cases := []struct {
			title      string
			scriptPath string
			scriptArgs []string
			expected   string
			wantSudo   bool
			envs       map[string]string
			wantErr    bool
			err        string
		}{
			{
				title:      "Happy case",
				scriptPath: testFile.Name(),
				scriptArgs: []string{"one"},
				expected:   "provided: one\n",
				wantSudo:   false,
				wantErr:    false,
			},
			{
				title:      "Happy case with sudo",
				scriptPath: testFile.Name(),
				scriptArgs: []string{"one"},
				expected:   "SUDO-SUCCESS\nprovided: one\n",
				wantSudo:   true,
				wantErr:    false,
			},
			{
				title:      "Error by remote script execution",
				scriptPath: testFile.Name(),
				scriptArgs: []string{"one", "two"},
				wantSudo:   false,
				wantErr:    true,
				err:        "execute on remote",
			},
			{
				title:      "With envs",
				scriptPath: testFile.Name(),
				scriptArgs: []string{"one"},
				expected:   "provided: one\n",
				wantSudo:   false,
				envs:       envs,
				wantErr:    false,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				s := sshClient.UploadScript(c.scriptPath, c.scriptArgs...)
				if c.wantSudo {
					s.Sudo()
				}
				if len(c.envs) > 0 {
					s.WithEnvs(c.envs)
				}
				out, err := s.Execute(context.Background())
				if !c.wantErr {
					require.NoError(t, err)
					require.Equal(t, c.expected, string(out))
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})

}

func TestUploadScriptExecuteBundle(t *testing.T) {
	testName := "TestUploadScriptExecuteBundle"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20026, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	// creating direactory to upload
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err = os.MkdirAll(testDir, 0755)
	if err != nil {
		// cannot start test w/o files to upload
		return
	}
	err = ssh_testing.PrepareFakeBashibleBundle(testDir, "test.sh", "bashible")
	require.NoError(t, err)

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20026"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		os.RemoveAll(testDir)
	})
	// we don't have /opt/deckhouse in the container, so we should create it before start any UploadScript with sudo
	err = container.CreateDeckhouseDirs()
	require.NoError(t, err)
	// in tests, app.TmpDirName doesn't exist
	app.TmpDirName = os.TempDir()

	t.Run("Upload script to container via existing ssh client", func(t *testing.T) {
		cases := []struct {
			title       string
			scriptPath  string
			scriptArgs  []string
			parentDir   string
			bundleDir   string
			prepareFunc func() error
			wantErr     bool
			err         string
		}{
			{
				title:      "Happy case",
				scriptPath: "test.sh",
				scriptArgs: []string{},
				parentDir:  testDir,
				bundleDir:  "bashible",
				wantErr:    false,
			},
			{
				title:      "Bundle error",
				scriptPath: "test.sh",
				scriptArgs: []string{"--add-failure"},
				parentDir:  testDir,
				bundleDir:  "bashible",
				wantErr:    true,
			},
			{
				title:      "Wrong bundle directory",
				scriptPath: "test.sh",
				scriptArgs: []string{},
				parentDir:  "/path/to/nonexistent/dir",
				bundleDir:  "wrong_bundle",
				wantErr:    true,
				err:        "tar bundle: failed to walk path",
			},
			{
				title:      "Upload error",
				scriptPath: "test.sh",
				scriptArgs: []string{""},
				parentDir:  testDir,
				bundleDir:  "bashible",
				prepareFunc: func() error {
					cmd := sshClient.Command("chmod", "700", app.DeckhouseNodeTmpPath)
					cmd.Sudo(context.Background())
					return cmd.Run(context.Background())
				},
				wantErr: true,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				s := sshClient.UploadScript(c.scriptPath, c.scriptArgs...)
				parentDir := c.parentDir
				bundleDir := c.bundleDir
				if c.prepareFunc != nil {
					err = c.prepareFunc()
					require.NoError(t, err)
				}
				_, err := s.ExecuteBundle(context.Background(), parentDir, bundleDir)
				if !c.wantErr {
					require.NoError(t, err)
					// require.Equal(t, c.expected, string(out))
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})

}
