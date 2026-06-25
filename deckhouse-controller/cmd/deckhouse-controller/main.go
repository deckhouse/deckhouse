// Copyright 2021 Flant JSC
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

package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	ad_app "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtolog"
	"github.com/flant/kube-client/klogtolog"
	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	"github.com/spf13/cobra"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/registry"
	dhctl_commands "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	dhctl_app "github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Variables with component versions. They set by 'go build' command.
var (
	DeckhouseVersion     = "dev"
	AddonOperatorVersion = "dev"
	ShellOperatorVersion = "dev"
	NelmVersion          = "dev"
)

// Variables to configure with build flags.
var (
	DefaultReleaseChannel = ""
)

const (
	defaultReleaseChannel = "Stable"
)

func version() string {
	return fmt.Sprintf("deckhouse %s (addon-operator %s, shell-operator %s, nelm %s, Golang %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion, NelmVersion, runtime.Version())
}

// main is almost a copy from addon-operator. We compile addon-operator to inline
// Go hooks and set some defaults. Also, helper commands are defined for Shell hooks.

const (
	AppName        = "deckhouse"
	AppDescription = "controller for Kubernetes platform from Flant"
)

// legacyBashCompletion is bound to the backward-compatibility flag
// `--completion-script-bash` (see rootCmd setup in main).
var legacyBashCompletion bool

func main() {
	sh_app.Version = ShellOperatorVersion
	ad_app.Version = AddonOperatorVersion

	// deckhouse-controller is the single source of truth for environment-driven
	// configuration of addon-operator (and the shell-operator globals
	// addon-operator manages). We start from addon-operator's hardcoded
	// defaults, then layer the env vars promised by the deckhouse-controller
	// deployment manifest on top via envconfig.Load. addon-operator's own
	// ParseEnv is intentionally not called: upstream renames (e.g. addon-operator
	// v1.21 moved MODULES_DIR under ADDON_OPERATOR_MODULES_DIR) must not
	// silently change the deckhouse env contract.
	cfg := ad_app.NewConfig()
	if err := envconfig.Load(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Mirror cfg into the addon-operator / shell-operator package-level globals
	// before registering debug sub-commands (queue, hook, global, module, raw).
	// Those sub-commands bind --debug-unix-socket to ad_app.DebugUnixSocket /
	// sh_app.DebugUnixSocket and dial them via DefaultClient(); without this
	// bridge a CLI invocation like `deckhouse-controller queue list` defaults
	// to /var/run/shell-operator/debug.socket while the running operator
	// actually listens on cfg.Debug.UnixSocket (set by the DEBUG_UNIX_SOCKET
	// env var in modules/002-deckhouse/templates/deployment.yaml). The `start`
	// command flow also performs this bridge inside NewAddonOperator, but for
	// non-start invocations NewAddonOperator never runs. This mirrors
	// addon-operator's own cmd/addon-operator/main.go which does the same
	// shapp.ApplyConfig(addon_operator.ShellOperatorConfig(cfg)) call before
	// debug.DefineDebugCommands(rootCmd) below.
	ad_app.ApplyConfig(cfg)
	sh_app.ApplyConfig(addonoperator.ShellOperatorConfig(cfg))

	logger := log.NewLogger()
	log.SetDefault(logger)

	fileName := filepath.Base(os.Args[0])

	rootCmd := &cobra.Command{
		Use:   fileName,
		Short: fmt.Sprintf("%s %s: %s", AppName, DeckhouseVersion, AppDescription),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Backward-compatibility alias for the legacy kingpin flag
			// `--completion-script-bash`, which was replaced by the cobra
			// `completion bash` subcommand after the migration to cobra.
			// External callers (and our own image's /etc/bashrc) still invoke
			// `deckhouse-controller --completion-script-bash`, so keep emitting
			// the bash completion script and exit early when the flag is set.
			// Use GenBashCompletionV2 (with descriptions) so the output is
			// byte-for-byte identical to the `completion bash` subcommand.
			if legacyBashCompletion {
				if err := cmd.Root().GenBashCompletionV2(os.Stdout, true); err != nil {
					return err
				}

				os.Exit(0)
			}

			klogtolog.InitAdapter(cfg.Debug.KubernetesAPI, logger.Named("klog"))
			stdliblogtolog.InitAdapter(logger)

			return nil
		},
	}

	// Legacy kingpin completion flag alias. Hidden from help output: the
	// canonical interface is the `completion` subcommand, this only preserves
	// backward compatibility for `--completion-script-bash`.
	rootCmd.PersistentFlags().BoolVar(&legacyBashCompletion, "completion-script-bash", false,
		"Generate the bash autocompletion script (alias for `completion bash`).")
	if err := rootCmd.PersistentFlags().MarkHidden("completion-script-bash"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to hide completion-script-bash flag: %v\n", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version.",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(version())
		},
	})

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start deckhouse.",
		RunE:  start(logger, cfg),
	}
	ad_app.BindFlags(cfg, rootCmd, startCmd)
	rootCmd.AddCommand(startCmd)

	// Add debug commands from shell-operator and addon-operator.
	sh_debug.DefineDebugCommands(rootCmd)
	ad_app.DefineDebugCommands(rootCmd)

	// Add more commands to the "module" command registered by addon-operator above.
	debug.DefineModuleConfigDebugCommands(rootCmd, logger)

	// deckhouse-controller helper subcommands.
	helpers.DefineHelperCommands(rootCmd, logger)

	// deckhouse-controller requirements.
	debug.DefineRequirementsCommands(rootCmd)

	// deckhouse-controller packages.
	debug.DefinePackagesCommands(rootCmd)

	// deckhouse-controller registry.
	registry.DefineRegistryCommand(rootCmd, logger)

	// dhctl command builders in dhctl/cmd/dhctl/commands/{edit,config}.go are
	// kingpin-based and rely on dhctl/pkg/app package-level globals. We seed
	// those globals from deployer-controlled env vars and bridge each dhctl
	// command into the cobra root via stub commands with DisableFlagParsing
	// that delegate the remaining argv to a kingpin Application built on the
	// fly.
	{
		dhctl_app.LoggerType = envOr("DECKHOUSE_LOGGER_TYPE", "json")
		dhctl_app.Editor = envOr("DECKHOUSE_EDITOR", "vim")
		dhctl_app.KubeConfigInCluster = envBoolOr("DECKHOUSE_KUBE_CONFIG_IN_CLUSTER", true)
		dhctl_app.TmpDirName = envOr("DECKHOUSE_TMP_DIR", os.TempDir())

		rootCmd.AddCommand(newDhctlBridge(
			"edit",
			"Change configuration files in Kubernetes cluster conveniently and safely.",
			fileName,
			func(kpApp *kingpin.Application) {
				editCmd := kpApp.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
				dhctl_commands.DefineEditCommands(editCmd /* wConnFlags */, true)
			},
		))

		rootCmd.AddCommand(newDhctlBridge(
			"cluster-configuration",
			"Parse configuration and print it.",
			fileName,
			func(kpApp *kingpin.Application) {
				dhctl_commands.DefineCommandParseClusterConfiguration(
					kpApp.Command("cluster-configuration", "Parse configuration and print it."),
				)
			},
		))

		rootCmd.AddCommand(newDhctlBridge(
			"cloud-discovery-data",
			"Parse cloud discovery data and print it.",
			fileName,
			func(kpApp *kingpin.Application) {
				dhctl_commands.DefineCommandParseCloudDiscoveryData(
					kpApp.Command("cloud-discovery-data", "Parse cloud discovery data and print it."),
				)
			},
		))
	}

	// Make "start" the default action when no subcommand is given.
	rootCmd.RunE = start(logger, cfg)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// envOr returns the env var name's value, or defaultValue when unset/empty.
func envOr(name, defaultValue string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return defaultValue
}

// envBoolOr parses the env var as a bool (per strconv.ParseBool), or returns
// defaultValue when unset, empty, or unparseable.
func envBoolOr(name string, defaultValue bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// newDhctlBridge builds a cobra stub that delegates parsing of its own argv
// tail to a freshly created kingpin Application.
//
// Flag parsing is disabled so cobra does not steal --help (kingpin prints its
// own usage for these subcommands) nor fail on dhctl-style flags that cobra
// knows nothing about.
func newDhctlBridge(name, short, fileName string, register func(*kingpin.Application)) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              short,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(_ *cobra.Command, _ []string) error {
			kpApp := kingpin.New(fileName, "")
			register(kpApp)
			if _, err := kpApp.Parse(os.Args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s, try --help\n", name, err)
				return err
			}
			return nil
		},
	}
}
