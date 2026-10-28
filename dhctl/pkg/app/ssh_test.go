// Copyright 2026 Flant JSC
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

package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/session"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

const sshTestLocalUser = "local"

type sshCase struct {
	name string
	env  map[string]string
	args []string
}

// parseSSHFlags runs `bootstrap` with SSH flags the way dhctl does: the environment through
// ApplyEnvironment in the root PersistentPreRunE.
func parseSSHFlags(t *testing.T, tc sshCase) (*options.Options, error) {
	t.Helper()
	t.Setenv("USER", sshTestLocalUser)
	t.Setenv("HOME", t.TempDir())

	for name, value := range tc.env {
		t.Setenv(name, value)
	}

	opts := options.New()
	opts.Global.CandiDir = "../../../candi/openapi/dhctl"
	opts.Global.TmpDir = t.TempDir()
	root := &cobra.Command{Use: "dhctl", SilenceUsage: true, SilenceErrors: true}
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return ApplyEnvironment(cmd)
	}
	cmd := &cobra.Command{Use: "bootstrap", RunE: func(*cobra.Command, []string) error { return nil }}
	DefineSSHFlags(cmd, opts)
	DefineBecomeFlags(cmd, &opts.Become)
	root.AddCommand(cmd)

	root.SetArgs(append([]string{"bootstrap"}, tc.args...))
	err := root.Execute()

	return opts, err
}

// writeConnectionConfig writes a connection config without keys; the schema is read from candi.
func writeConnectionConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "connection-config.yaml")
	body := "apiVersion: dhctl.deckhouse.io/v1\nkind: SSHConfig\nsshUser: ubuntu\nsudoPassword: dh-test-secret-0123456789abcdef\n---\napiVersion: dhctl.deckhouse.io/v1\nkind: SSHHost\nhost: 10.0.0.1\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func TestSSHBastionUserFollowsUser(t *testing.T) {
	cases := []struct {
		sshCase
		want string
	}{
		{sshCase{name: "user from env", env: map[string]string{"DHCTL_CLI_SSH_USER": "ubuntu"}}, "ubuntu"},
		{sshCase{name: "user on the command line", args: []string{"--ssh-user", "ubuntu"}}, "ubuntu"},
		{sshCase{
			name: "explicit bastion user equal to the default stays",
			args: []string{"--ssh-bastion-user", sshTestLocalUser, "--ssh-user", "ubuntu"},
		}, sshTestLocalUser},
		{sshCase{
			name: "bastion user from env equal to the default stays",
			env:  map[string]string{"DHCTL_CLI_SSH_BASTION_USER": sshTestLocalUser},
			args: []string{"--ssh-user", "ubuntu"},
		}, sshTestLocalUser},
		{sshCase{
			name: "bastion user from env is not overwritten by the command line",
			env:  map[string]string{"DHCTL_CLI_SSH_BASTION_USER": "jump"},
			args: []string{"--ssh-user", "ubuntu"},
		}, "jump"},
		{sshCase{name: "nothing given"}, sshTestLocalUser},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := parseSSHFlags(t, tc.sshCase)
			require.NoError(t, err)
			require.Equal(t, tc.want, o.SSH.BastionUser)
		})
	}
}

// --connection-config replaces the SSH flags; given together, they cannot be told apart, so that
// is refused wherever the flags come from.
func TestConnectionConfigExcludesSSHFlags(t *testing.T) {
	const exclusive = "'connection-config' cannot be specified with other ssh flags at the same time"

	conflicting := []sshCase{
		{name: "hosts from env", env: map[string]string{"DHCTL_CLI_SSH_HOSTS": "h1"}},
		{name: "user from env", env: map[string]string{"DHCTL_CLI_SSH_USER": "ubuntu"}},
		{name: "port on the command line", args: []string{"--ssh-port", "2222"}},
		{name: "port equal to the default on the command line", args: []string{"--ssh-port", "22"}},
		// Any existing file passes the key path check; the key is never read here.
		{name: "key from env", env: map[string]string{"DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS": "ssh_test.go"}},
	}
	for _, tc := range conflicting {
		t.Run(tc.name, func(t *testing.T) {
			tc.args = append(tc.args, "--connection-config", writeConnectionConfig(t))
			_, err := parseSSHFlags(t, tc)
			require.EqualError(t, err, exclusive)
		})
	}
}

// The file of --connection-config, from argv or its variable, gives the connection and the sudo password.
func TestConnectionConfigIsRead(t *testing.T) {
	check := func(t *testing.T, tc sshCase) {
		t.Helper()
		o, err := parseSSHFlags(t, tc)
		require.NoError(t, err)
		require.Equal(t, "ubuntu", o.SSH.User)
		require.Equal(t, []session.Host{{Host: "10.0.0.1", Name: "0"}}, o.SSH.Hosts)
		require.Equal(t, "dh-test-secret-0123456789abcdef", o.Become.BecomePass)
	}
	t.Run("command line", func(t *testing.T) {
		check(t, sshCase{args: []string{"--connection-config", writeConnectionConfig(t)}})
	})
	t.Run("env", func(t *testing.T) {
		check(t, sshCase{env: map[string]string{"DHCTL_CLI_CONNECTION_CONFIG": writeConnectionConfig(t)}})
	})
}

// A connection config without keys leaves none: the default key is filled in first, then the
// file replaces the keys.
func TestConnectionConfigReplacesDefaultKey(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "id_rsa"), nil, 0o600))

	o, err := parseSSHFlags(t, sshCase{env: map[string]string{"HOME": home}})
	require.NoError(t, err)
	require.Equal(t, []sshconfig.AgentPrivateKey{{Key: filepath.Join(home, ".ssh", "id_rsa"), IsPath: true}}, o.SSH.PrivateKeys, "the default key without a connection config")

	o, err = parseSSHFlags(t, sshCase{
		env:  map[string]string{"HOME": home},
		args: []string{"--connection-config", writeConnectionConfig(t)},
	})
	require.NoError(t, err)
	require.Empty(t, o.SSH.PrivateKeys)
}

// lib-connection reads nothing from the process: the tmp dir, the agent socket and the kube-proxy
// knobs Deckhouse Commander sets reach it only through these params.
func TestProviderParamsCarryWhatTheProcessWasGiven(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/run/agent.sock")
	t.Setenv("KUBE_PROXY_ACCEPT_HOSTS", "^.*$")
	t.Setenv("KUBE_PROXY_BIND_ADDR", "0.0.0.0")
	tmpDir := t.TempDir()

	params := ProviderParams(&options.GlobalOptions{TmpDir: tmpDir}, slog.New(slog.DiscardHandler))

	require.Equal(t, tmpDir, params.TmpDir)
	require.Equal(t, "/run/agent.sock", params.AuthSock)
	require.Equal(t, "^.*$", params.KubeProxyAcceptHosts)
	require.Equal(t, "0.0.0.0", params.KubeProxyBindAddr)
}
