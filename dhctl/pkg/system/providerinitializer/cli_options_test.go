// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/kube"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// testSecret is the allowlisted test value of AGENTS.md: the sudo password and the key passphrase.
const testSecret = "dh-test-secret-0123456789abcdef"

// cliRun is one dhctl run of `bootstrap`: its environment and its argv.
type cliRun struct {
	name string
	env  map[string]string
	args []string
}

// runBootstrap applies argv and the environment the way dhctl does, then calls run
// with the options every flag hook has seen. $HOME holds no key: the default key is missing.
func runBootstrap(t *testing.T, tc cliRun, run func(ctx context.Context, opts *options.Options) error) error {
	t.Helper()
	t.Setenv("USER", "local")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KUBECONFIG", "")

	for name, value := range tc.env {
		t.Setenv(name, value)
	}

	opts := options.New()
	opts.Global.CandiDir = "../../../../candi/openapi/dhctl"
	opts.Global.TmpDir = t.TempDir()
	root := &cobra.Command{Use: "dhctl", SilenceUsage: true, SilenceErrors: true}
	// As cmd/dhctl does: the environment fills what argv left unset.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return app.ApplyEnvironment(cmd)
	}
	cmd := &cobra.Command{Use: "bootstrap", RunE: func(cmd *cobra.Command, _ []string) error {
		return run(cmd.Context(), opts)
	}}
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)
	root.AddCommand(cmd)

	root.SetArgs(append([]string{"bootstrap"}, tc.args...))

	return root.ExecuteContext(t.Context())
}

// cliConnection is the connection config a command hands NewSSHProviderInitializer.
func cliConnection(t *testing.T, tc cliRun) (*sshconfig.ConnectionConfig, *options.Options, error) {
	t.Helper()

	var (
		cfg    *sshconfig.ConnectionConfig
		parsed *options.Options
	)

	err := runBootstrap(t, tc, func(ctx context.Context, opts *options.Options) error {
		parsed = opts

		initializer, err := CLISSHProviderInitializer(ctx, opts)
		if err != nil {
			return err
		}

		cfg = initializer.GetConfig()

		return nil
	})

	return cfg, parsed, err
}

func newPrivateKey(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "id_ed25519")
	writePrivateKey(t, path)

	return path
}

func hostsOf(cfg *sshconfig.ConnectionConfig) []string {
	hosts := make([]string, 0, len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		hosts = append(hosts, h.Host)
	}

	return hosts
}

// encryptedPrivateKey is a fresh ed25519 key in PEM, encrypted with testSecret.
func encryptedPrivateKey(t *testing.T) []byte {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKeyWithPassphrase(privateKey, "", []byte(testSecret))
	require.NoError(t, err)

	return pem.EncodeToMemory(block)
}

// writeConnectionConfig writes a --connection-config file: an SSHConfig with an encrypted key as
// text, its passphrase, and one SSHHost.
func writeConnectionConfig(t *testing.T) string {
	t.Helper()

	key := encryptedPrivateKey(t)
	path := filepath.Join(t.TempDir(), "connection.yaml")
	body := "apiVersion: dhctl.deckhouse.io/v1\nkind: SSHConfig\nsshUser: ubuntu\nsudoPassword: " + testSecret + "\nsshAgentPrivateKeys:\n- key: |\n" +
		"    " + strings.ReplaceAll(strings.TrimSpace(string(key)), "\n", "\n    ") + "\n  passphrase: " + testSecret + "\n" +
		"---\napiVersion: dhctl.deckhouse.io/v1\nkind: SSHHost\nhost: h1\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

// Hosts and keys are split on commas from every source, as lib-connection's StringSlice flag
// and its environment lists did.
func TestCLIConnectionSplitsLists(t *testing.T) {
	key := newPrivateKey(t)
	other := newPrivateKey(t)

	cases := []struct {
		cliRun
		hosts []string
		keys  []string
	}{
		{cliRun{name: "argv", args: []string{"--ssh-host", "a,b", "--ssh-agent-private-keys", key}}, []string{"a", "b"}, []string{key}},
		{cliRun{name: "argv repeated", args: []string{"--ssh-host", "a", "--ssh-host", "b,c", "--ssh-agent-private-keys", key}}, []string{"a", "b", "c"}, []string{key}},
		{cliRun{name: "argv quoted", args: []string{"--ssh-host", `"a",b`, "--ssh-agent-private-keys", key}}, []string{"a", "b"}, []string{key}},
		{cliRun{name: "argv keys", args: []string{"--ssh-host", "a", "--ssh-agent-private-keys", key + "," + other + "," + key}}, []string{"a"}, []string{key, other}},
		{cliRun{name: "env", env: map[string]string{"DHCTL_CLI_SSH_HOSTS": "a,b", "DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS": key + "," + other}}, []string{"a", "b"}, []string{key, other}},
		{cliRun{name: "env blank items", env: map[string]string{"DHCTL_CLI_SSH_HOSTS": "a,,b, ", "DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS": key}}, []string{"a", "b"}, []string{key}},
		{cliRun{name: "env lines", env: map[string]string{"DHCTL_CLI_SSH_HOSTS": "a\nb,c", "DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS": key}}, []string{"a", "b", "c"}, []string{key}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _, err := cliConnection(t, tc.cliRun)
			require.NoError(t, err)
			require.Equal(t, tc.hosts, hostsOf(cfg))

			keys := make([]string, 0, len(cfg.Config.PrivateKeys))
			for _, k := range cfg.Config.PrivateKeys {
				keys = append(keys, k.Key)
			}

			require.Equal(t, tc.keys, keys)
		})
	}

	t.Run("a host given twice", func(t *testing.T) {
		_, _, err := cliConnection(t, cliRun{args: []string{"--ssh-host", "a,b,a", "--ssh-agent-private-keys", key}})
		require.EqualError(t, err, "extract config: host 'a' present multiple times 2")
	})
}

func TestCLIConnectionWithoutPrivateKeys(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "agent.sock")
	require.NoError(t, os.WriteFile(sock, nil, 0o600))
	// The default key is there: only refusing keys keeps it out.
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
	writePrivateKey(t, filepath.Join(home, ".ssh", "id_rsa"))

	agentOnly := []cliRun{
		{name: "argv", args: []string{"--ssh-host", "a", "--force-no-private-keys", "--use-agent-with-no-private-keys"}},
		{name: "env", env: map[string]string{
			"DHCTL_CLI_SSH_HOSTS":                      "a",
			"DHCTL_CLI_FORCE_NO_PRIVATE_KEYS":          "true",
			"DHCTL_CLI_USE_AGENT_WITH_NO_PRIVATE_KEYS": "true",
		}},
		{name: "keys variable set but empty", env: map[string]string{
			"DHCTL_CLI_SSH_HOSTS":                      "a",
			"DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS":         "",
			"DHCTL_CLI_USE_AGENT_WITH_NO_PRIVATE_KEYS": "true",
		}},
	}
	for _, tc := range agentOnly {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SSH_AUTH_SOCK", sock)

			env := map[string]string{"HOME": home}
			maps.Copy(env, tc.env)
			tc.env = env
			cfg, opts, err := cliConnection(t, tc)
			require.NoError(t, err)
			require.Empty(t, cfg.Config.PrivateKeys)
			require.True(t, cfg.Config.ForceUseSSHAgent)
			require.Empty(t, cfg.Config.SudoPassword)
			require.Empty(t, opts.SSH.PrivateKeys)
		})
	}

	t.Run("agent socket missing", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "absent.sock"))
		_, _, err := cliConnection(t, cliRun{args: []string{"--ssh-host", "a", "--force-no-private-keys", "--use-agent-with-no-private-keys"}})
		require.ErrorContains(t, err, "cannot get auth socket from env SSH_AUTH_SOCK file info for")
	})

	t.Run("default key missing", func(t *testing.T) {
		_, _, err := cliConnection(t, cliRun{args: []string{"--ssh-host", "a"}})
		require.ErrorContains(t, err, "extract config: Failed to read private keys from flags:")
		require.ErrorContains(t, err, filepath.Join(".ssh", "id_rsa"))
	})

	t.Run("default key missing with kube flags gives no ssh", func(t *testing.T) {
		kubeconfig := writeKubeconfig(t, newFakeAPIServer(t, false).URL)

		var (
			initializer  *SSHProviderInitializer
			kubeProvider libcon.KubeProvider
		)

		err := runBootstrap(t, cliRun{args: []string{"--kubeconfig", kubeconfig}}, func(ctx context.Context, opts *options.Options) error {
			var err error

			initializer, kubeProvider, err = CLIProviders(ctx, opts, CLIProvidersOptions{})

			return err
		})
		require.NoError(t, err)
		require.Nil(t, initializer)
		require.NotNil(t, kubeProvider)
	})
}

// The kube flag's variable reaches the kube config through the parsed options: a CLI command never
// re-parses os.Args and the environment for it.
func TestCLIKubeConfigFromEnvironment(t *testing.T) {
	apiserver := newFakeAPIServer(t, false)
	kubeconfig := writeKubeconfig(t, apiserver.URL)

	var cfg *kube.Config

	err := runBootstrap(t, cliRun{env: map[string]string{"DHCTL_CLI_KUBE_CONFIG": kubeconfig}}, func(ctx context.Context, opts *options.Options) error {
		var err error

		cfg, err = impersonateKubeConfig(ctx, &kube.Config{KubeConfig: opts.Kube.Config})

		return err
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.RestConfig)
	require.Equal(t, apiserver.URL, cfg.RestConfig.Host)
}

// The --connection-config file reaches the connection: the hosts, the user, the sudo password and
// the passphrase of its key, which is not asked for in the terminal.
func TestCLIConnectionFromConnectionConfigFile(t *testing.T) {
	cfg, opts, err := cliConnection(t, cliRun{args: []string{"--connection-config", writeConnectionConfig(t)}})
	require.NoError(t, err)

	require.Equal(t, []string{"h1"}, hostsOf(cfg))
	require.Equal(t, "ubuntu", cfg.Config.User)
	require.Equal(t, testSecret, cfg.Config.SudoPassword)
	require.Len(t, cfg.Config.PrivateKeys, 1)
	require.Equal(t, testSecret, cfg.Config.PrivateKeys[0].Passphrase)
	require.Equal(t, "ubuntu", opts.SSH.User)
}

// A --connection-config file without keys is valid: the sudo password is the way in, and the
// missing default key is not looked for, as lib-connection read the file.
func TestCLIConnectionFromConnectionConfigFileWithoutKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connection.yaml")
	body := "apiVersion: dhctl.deckhouse.io/v1\nkind: SSHConfig\nsshUser: ubuntu\nsudoPassword: " + testSecret + "\n" +
		"---\napiVersion: dhctl.deckhouse.io/v1\nkind: SSHHost\nhost: h1\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	cfg, _, err := cliConnection(t, cliRun{args: []string{"--connection-config", path}})
	require.NoError(t, err)

	require.Equal(t, []string{"h1"}, hostsOf(cfg))
	require.Empty(t, cfg.Config.PrivateKeys)
	require.Equal(t, testSecret, cfg.Config.SudoPassword)
}

// --connection-config gives the keys exactly: a refusal of the default key next to it is refused.
func TestCLIConnectionConfigRefusesNoPrivateKeys(t *testing.T) {
	for name, tc := range map[string]cliRun{
		"empty keys variable": {env: map[string]string{"DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS": ""}},
		"flag":                {args: []string{"--force-no-private-keys"}},
		"variable":            {env: map[string]string{"DHCTL_CLI_FORCE_NO_PRIVATE_KEYS": "true"}},
	} {
		t.Run(name, func(t *testing.T) {
			tc.args = append(tc.args, "--connection-config", writeConnectionConfig(t))
			_, _, err := cliConnection(t, tc)
			require.EqualError(t, err, "'connection-config' cannot be specified with --force-no-private-keys or an empty DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS")
		})
	}
}

// A passphrase from the terminal that does not open the key is reported with the cause: a cancelled
// context or an unsupported key must not read as a mistyped passphrase.
func TestWrongAskedPassphraseKeepsTheCause(t *testing.T) {
	ask := func(context.Context, string) ([]byte, error) { return []byte("dh-test-secret-wrong"), nil }

	_, err := readPrivateKey(t.Context(), "k", encryptedPrivateKey(t), "", ask)
	require.ErrorContains(t, err, `wrong passphrase for ssh key "k"`)
	require.ErrorIs(t, err, x509.IncorrectPasswordError)
}
