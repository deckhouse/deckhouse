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

// Bridge from cobra (used by deckhouse-controller's main.go) to the kingpin
// command builders in this package. The builders themselves stay kingpin-based
// because they are byte-for-byte mirrors of dhctl/cmd/dhctl/commands/{edit,
// config}.go (drift is enforced by tools/check-dhctl-cmd-drift.sh).
//
// For each dhctl-style top-level command we expose a cobra command whose flag
// parsing is disabled; cobra is used only to route argv to the correct kingpin
// Application, which then handles flag parsing, --help and execution.

package dhctlcli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/alecthomas/kingpin.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	clusterConfigurationSecretNS   = "kube-system"
	clusterConfigurationSecretName = "d8-cluster-configuration"
	clusterConfigurationSecretKey  = "cluster-configuration.yaml"
)

// RegisterCobraBridges adds cobra wrappers for the kingpin-based dhctl
// commands: `edit`, `cluster-configuration` and `cloud-discovery-data`.
//
// fileName is the argv[0] basename used as the kingpin application name, so
// `--help` output matches the binary the user invoked.
func RegisterCobraBridges(rootCmd *cobra.Command, fileName string, opts *options.Options) {
	rootCmd.AddCommand(newDhctlBridge(
		"edit",
		"Change configuration files in Kubernetes cluster conveniently and safely.",
		fileName,
		nil,
		func(kpApp *kingpin.Application) {
			editCmd := kpApp.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
			DefineEditCommands(editCmd, opts /* wConnFlags */, true)
		},
	))

	rootCmd.AddCommand(newDhctlBridge(
		"cluster-configuration",
		"Parse configuration and print it.",
		fileName,
		// When invoked interactively (a TTY) with neither --file nor piped
		// stdin, read the cluster-configuration straight from the cluster so
		// the command prints the current config instead of blocking on stdin.
		clusterConfigurationStdinFromCluster(opts),
		func(kpApp *kingpin.Application) {
			DefineCommandParseClusterConfiguration(
				kpApp.Command("cluster-configuration", "Parse configuration and print it."),
				opts,
			)
		},
	))

	rootCmd.AddCommand(newDhctlBridge(
		"cloud-discovery-data",
		"Parse cloud discovery data and print it.",
		fileName,
		nil,
		func(kpApp *kingpin.Application) {
			DefineCommandParseCloudDiscoveryData(
				kpApp.Command("cloud-discovery-data", "Parse cloud discovery data and print it."),
				opts,
			)
		},
	))
}

// newDhctlBridge builds a cobra stub that delegates parsing of its own argv
// tail to a freshly created kingpin Application.
//
// We must DisableFlagParsing so that cobra does not steal --help (kingpin
// prints its own usage for these subcommands) or fail on dhctl-style flags
// that cobra knows nothing about.
//
// preRun, when non-nil, runs before the kingpin Application is parsed. It is
// used to prime stdin for commands that should read from the cluster when no
// explicit input is given.
func newDhctlBridge(name, short, fileName string, preRun func() error, register func(*kingpin.Application)) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              short,
		DisableFlagParsing: true,
		// Suppress cobra's "unknown flag" / arg validation; everything past
		// the command name is forwarded to kingpin.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if preRun != nil {
				if err := preRun(); err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", name, err)
					return err
				}
			}

			kpApp := kingpin.New(fileName, "")
			register(kpApp)
			if _, err := kpApp.Parse(os.Args[1:]); err != nil {
				// Mirror kingpin.MustParse formatting so users see the same
				// guidance they would from the standalone kingpin app.
				fmt.Fprintf(os.Stderr, "%s: %s, try --help\n", name, err)
				return err
			}
			return nil
		},
	}
}

// clusterConfigurationStdinFromCluster returns a preRun that feeds the
// in-cluster d8-cluster-configuration secret to the `cluster-configuration`
// parse command via stdin.
//
// It only fires when the user provided no input source: no --file flag (or
// DHCTL_CLI_FILE env) and stdin is an interactive terminal rather than a pipe.
// In every other case it is a no-op, preserving the original behavior of
// reading --file or piped stdin.
func clusterConfigurationStdinFromCluster(opts *options.Options) func() error {
	return func() error {
		if parseFileFlagPresent(os.Args) || os.Getenv("DHCTL_CLI_FILE") != "" || !input.IsTerminal() {
			return nil
		}

		ctx := context.Background()

		kubeCl, cleanup, err := newKubeClient(ctx, opts)
		if err != nil {
			return err
		}
		defer cleanup(ctx)

		secret, err := kubeCl.CoreV1().Secrets(clusterConfigurationSecretNS).
			Get(ctx, clusterConfigurationSecretName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get %s secret: %w", clusterConfigurationSecretName, err)
		}

		data, ok := secret.Data[clusterConfigurationSecretKey]
		if !ok || len(data) == 0 {
			return fmt.Errorf("secret %s has no %q key", clusterConfigurationSecretName, clusterConfigurationSecretKey)
		}

		// Hand the secret bytes to the parse action via stdin without touching
		// disk (the data is sensitive). The mirrored parse function reads
		// os.Stdin when no --file is set.
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		os.Stdin = r
		go func() {
			_, _ = w.Write(data)
			_ = w.Close()
		}()

		return nil
	}
}

// newKubeClient builds a Kubernetes client from the deckhouse-controller's
// dhctl options. It mirrors the client setup used by the edit commands so that
// in-cluster (the deckhouse pod) and kubeconfig-based access behave the same.
func newKubeClient(ctx context.Context, opts *options.Options) (*client.KubernetesClient, func(context.Context), error) {
	logger := log.GetDefaultLogger()
	loggerProvider := log.ExternalLoggerProvider(logger)
	params := app.ProviderParams(&opts.Global, loggerProvider)

	sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
		ctx,
		params,
		providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
		providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
		providerinitializer.WithRequiredKubeProvider(),
	)
	if err != nil {
		return nil, nil, err
	}
	if kubeProvider == nil {
		return nil, nil, fmt.Errorf("kubernetes provider is not initialized")
	}

	kube, err := kubeProvider.Client(ctx)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func(c context.Context) {
		//nolint: errcheck
		if sshProviderInitializer != nil {
			sshProviderInitializer.Cleanup(c)
		}
	}

	return &client.KubernetesClient{KubeClient: kube}, cleanup, nil
}

// parseFileFlagPresent reports whether the args contain the parse commands'
// --file/-f flag in any of its accepted spellings.
func parseFileFlagPresent(args []string) bool {
	for _, a := range args {
		switch {
		case a == "-f", a == "--file":
			return true
		case strings.HasPrefix(a, "--file="), strings.HasPrefix(a, "-f="):
			return true
		}
	}
	return false
}
