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
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/configcmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	clusterConfigurationSecretNS   = "kube-system"
	clusterConfigurationSecretName = "d8-cluster-configuration"
	clusterConfigurationSecretKey  = "cluster-configuration.yaml"
)

// RegisterDhctlCommands adds the dhctl commands the controller ships, from the same package dhctl
// registers them from: edit, cluster-configuration, cloud-discovery-data.
func RegisterDhctlCommands(rootCmd *cobra.Command, opts *options.Options) {
	// A runnable group with NoArgs refuses a mistyped subcommand, and a bare one fails with
	// kingpin's error, as dhctl's buildRoot does.
	edit := &cobra.Command{
		Use:          "edit",
		Short:        "Change configuration files in Kubernetes cluster conveniently and safely.",
		Args:         cobra.NoArgs,
		RunE:         func(*cobra.Command, []string) error { return errors.New("must select a subcommand of 'edit'") },
		SilenceUsage: true,
	}
	configcmd.DefineKubeEditCommands(edit, opts)

	parseCluster := &cobra.Command{Use: "cluster-configuration", Short: "Parse configuration and print it.", Args: cobra.NoArgs}
	configcmd.DefineCommandParseClusterConfiguration(parseCluster, opts)

	parseCloud := &cobra.Command{Use: "cloud-discovery-data", Short: "Parse cloud discovery data and print it.", Args: cobra.NoArgs}
	configcmd.DefineCommandParseCloudDiscoveryData(parseCloud, opts)
	// The environment is the first PreRunE hook of every leaf: the flag groups above add none, and
	// the controller's root owns PersistentPreRunE. cobra reads SilenceUsage from the executing command.
	for _, leaf := range append([]*cobra.Command{parseCluster, parseCloud}, edit.Commands()...) {
		leaf.SilenceUsage = true
		app.AfterApply(leaf, app.ApplyEnvironment)
	}

	app.AfterApply(parseCluster, clusterConfigurationStdinFromCluster(opts))
	// On these commands only: the controller's own commands never had --help-long, kingpin's
	// flag error texts or variables in help.
	for _, c := range []*cobra.Command{edit, parseCluster, parseCloud} {
		c.SetGlobalNormalizationFunc(app.KingpinFlagNames)
		c.SetFlagErrorFunc(app.FlagError)
		app.ShowEnvInHelp(c)
	}

	rootCmd.AddCommand(edit, parseCluster, parseCloud)
}

// clusterConfigurationStdinFromCluster feeds the in-cluster d8-cluster-configuration secret to the
// parse command as its input when a human runs it with no input source: no --file or DHCTL_CLI_FILE,
// stdin a terminal. In every other case the command reads --file or piped stdin as before.
func clusterConfigurationStdinFromCluster(opts *options.Options) func(cmd *cobra.Command) error {
	return func(cmd *cobra.Command) error {
		if cmd.Flags().Changed("file") || !input.IsTerminal() {
			return nil
		}

		// A human views the config, so YAML, unless -o or DHCTL_CLI_OUTPUT asks otherwise.
		if !cmd.Flags().Changed("output") {
			opts.Render.ParseOutput = "yaml"
		}

		ctx := cmd.Context()

		kubeCl, err := newKubeClient(ctx, opts)
		if err != nil {
			return err
		}

		secret, err := kubeCl.CoreV1().Secrets(clusterConfigurationSecretNS).
			Get(ctx, clusterConfigurationSecretName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get %s secret: %w", clusterConfigurationSecretName, err)
		}

		data, ok := secret.Data[clusterConfigurationSecretKey]
		if !ok || len(data) == 0 {
			return fmt.Errorf("secret %s has no %q key", clusterConfigurationSecretName, clusterConfigurationSecretKey)
		}

		// The secret stays in memory, off disk; without --file the parse command reads cmd.InOrStdin().
		cmd.SetIn(bytes.NewReader(data))

		return nil
	}
}

// newKubeClient builds a client from the controller's kube options: in-cluster by default
// (DECKHOUSE_KUBE_CONFIG_IN_CLUSTER), an explicit kubeconfig if given. No SSH: it only reads a Secret.
func newKubeClient(ctx context.Context, opts *options.Options) (*client.KubernetesClient, error) {
	kubeCl := client.NewKubernetesClient()
	if err := kubeCl.InitContext(ctx, client.AppKubernetesInitParams(&opts.Kube)); err != nil {
		return nil, fmt.Errorf("initialize kubernetes client: %w", err)
	}

	return kubeCl, nil
}
