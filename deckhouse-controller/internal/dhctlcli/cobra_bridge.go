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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
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
		func(kpApp *kingpin.Application) {
			editCmd := kpApp.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
			DefineEditCommands(editCmd, opts /* wConnFlags */, true)
		},
	))

	rootCmd.AddCommand(newDhctlBridge(
		"cluster-configuration",
		"Parse configuration and print it.",
		fileName,
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
func newDhctlBridge(name, short, fileName string, register func(*kingpin.Application)) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              short,
		DisableFlagParsing: true,
		// Suppress cobra's "unknown flag" / arg validation; everything past
		// the command name is forwarded to kingpin.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
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
