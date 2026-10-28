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

package dhctlcli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
)

func TestExecuteErrors(t *testing.T) {
	root := &cobra.Command{Use: "deckhouse-controller"}
	RegisterDhctlCommands(root, options.New())

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	root.SetArgs([]string{"cloud-discovery-data", "--file", filepath.Join(t.TempDir(), "absent.yaml")})
	require.ErrorContains(t, root.Execute(), "loading input file")
	require.NotContains(t, out.String(), "Usage:")

	out.Reset()
	root.SetArgs([]string{"edit", "bogus"})
	require.ErrorContains(t, root.Execute(), `unknown command "bogus"`)
	require.NotContains(t, out.String(), "Usage:")

	// kingpin's text for a repeated flag, without pflag's "invalid argument" prefix.
	root.SetArgs([]string{"cluster-configuration", "--output", "json", "--output", "yaml"})
	require.EqualError(t, root.Execute(), "flag 'output' cannot be repeated")

	// kingpin refused a bare group with the error alone, and the controller exited 1.
	out.Reset()
	root.SetArgs([]string{"edit"})
	require.EqualError(t, root.Execute(), "must select a subcommand of 'edit'")
	require.NotContains(t, out.String(), "Usage:")
}

// kingpin had a hidden --help-long on every command; the controller's other commands never did.
func TestHelpLongPrintsHelp(t *testing.T) {
	root := &cobra.Command{Use: "deckhouse-controller"}
	root.AddCommand(&cobra.Command{Use: "other", Run: func(*cobra.Command, []string) {}})
	RegisterDhctlCommands(root, options.New())

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	edit, _, err := root.Find([]string{"edit"})
	require.NoError(t, err)

	commands := [][]string{{"edit"}, {"cluster-configuration"}, {"cloud-discovery-data"}}
	for _, sub := range edit.Commands() {
		commands = append(commands, []string{"edit", sub.Name()})
	}

	for _, args := range commands {
		out.Reset()
		root.SetArgs(append(args, "--help-long"))
		require.NoError(t, root.Execute(), args)
		require.Contains(t, out.String(), "Usage:", args)
	}

	root.SetArgs([]string{"other", "--help-long"})
	require.EqualError(t, root.Execute(), "unknown flag: --help-long")
}

// Help names the variable of a flag after its help text, as the kingpin help of these commands did.
func TestHelpNamesFlagVariables(t *testing.T) {
	root := &cobra.Command{Use: "deckhouse-controller"}
	RegisterDhctlCommands(root, options.New())

	var out bytes.Buffer
	root.SetOut(&out)

	root.SetArgs([]string{"edit", "cluster-configuration", "--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "($DHCTL_CLI_KUBE_CONFIG)")

	out.Reset()
	root.SetArgs([]string{"cluster-configuration", "--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "($DHCTL_CLI_OUTPUT)")
}

// Under go test stdin is not a terminal, so the in-cluster hook stays out and the leaf reads the
// root's input: the cmd.InOrStdin the hook feeds the secret into.
func TestClusterConfigurationReadsCommandInput(t *testing.T) {
	opts := options.New()
	opts.Global.CandiDir = tests.RequireCandiDir(t)
	// Run from a terminal the hook fires: the kubeconfig points nowhere, so no cluster is reached.
	opts.Kube.Config = filepath.Join(t.TempDir(), "absent")
	root := &cobra.Command{Use: "deckhouse-controller"}
	RegisterDhctlCommands(root, opts)

	var out bytes.Buffer

	root.SetIn(strings.NewReader(`apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
`))
	root.SetOut(&out)
	root.SetArgs([]string{"cluster-configuration"})

	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), `"clusterType":"Static"`)
}

// The flags see the environment, as dhctl's do. The SSH variables have no flag to reach.
func TestEnvReachesFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USER", "root")
	t.Setenv("DHCTL_CLI_KUBE_CONFIG", "/kubeconfig")
	t.Setenv("DHCTL_CLI_SSH_HOSTS", "10.0.0.1")
	t.Setenv("DHCTL_CLI_CONNECTION_CONFIG", "connection.yaml")
	t.Setenv("DHCTL_CLI_OUTPUT", "yaml")

	opts := options.New()
	root := &cobra.Command{Use: "deckhouse-controller"}
	RegisterDhctlCommands(root, opts)
	// --file keeps cluster-configuration off the in-cluster secret when stdin is a terminal.
	for _, args := range [][]string{{"edit", "cluster-configuration"}, {"cluster-configuration", "--file", "unused.yaml"}} {
		leaf, _, err := root.Find(args)
		require.NoError(t, err)

		leaf.RunE = func(*cobra.Command, []string) error { return nil }

		root.SetArgs(args)
		require.NoError(t, root.Execute())
	}

	require.Equal(t, "/kubeconfig", opts.Kube.Config)
	require.Empty(t, opts.SSH.HostsRaw)
	require.Empty(t, opts.SSH.ConnectionConfigPath)
	require.Equal(t, "yaml", opts.Render.ParseOutput)
}

// The controller runs in the cluster: its commands reach Kubernetes with the kube flags only and
// take no SSH, become or --config flags.
func TestNoSSHFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USER", "root")

	root := &cobra.Command{Use: "deckhouse-controller"}
	RegisterDhctlCommands(root, options.New())

	edit, _, err := root.Find([]string{"edit"})
	require.NoError(t, err)
	require.Len(t, edit.Commands(), 3)
	leaves := edit.Commands()

	for _, args := range [][]string{{"cluster-configuration"}, {"cloud-discovery-data"}} {
		leaf, _, err := root.Find(args)
		require.NoError(t, err)

		leaves = append(leaves, leaf)
	}

	for _, leaf := range leaves {
		leaf.Flags().VisitAll(func(f *pflag.Flag) {
			require.False(t, strings.HasPrefix(f.Name, "ssh-"), "%s --%s", leaf.CommandPath(), f.Name)
		})

		for _, name := range []string{"connection-config", "config", "ask-become-pass", "ask-bastion-pass", "force-no-private-keys", "use-agent-with-no-private-keys"} {
			require.Nil(t, leaf.Flags().Lookup(name), "%s --%s", leaf.CommandPath(), name)
		}
	}

	for _, leaf := range edit.Commands() {
		require.NotNil(t, leaf.Flags().Lookup("kubeconfig"), leaf.CommandPath())
		leaf.RunE = func(*cobra.Command, []string) error { return nil }
	}

	root.SetArgs([]string{"edit", "cluster-configuration", "--ssh-host", "x"})
	require.EqualError(t, root.Execute(), "unknown flag: --ssh-host")
}
