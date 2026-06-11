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

	ad_app "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtolog"
	"github.com/flant/kube-client/klogtolog"
	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/dhctlcli"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
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

	// Mirror cfg into addon-operator package-level globals and shell-operator's
	// debug.DefaultSocketPath before registering debug sub-commands (queue,
	// hook, global, module, raw).
	//
	// ad_app.ApplyConfig populates the addon-operator globals (ModulesDir,
	// Namespace, etc.) so that debug commands defined by addon-operator can
	// locate config paths. The `start` command flow also performs this bridge
	// inside NewAddonOperator, but for non-start invocations (e.g.
	// `deckhouse-controller queue list`) NewAddonOperator never runs.
	//
	// sh_debug.DefaultSocketPath is the CLI-side global that shell-operator's
	// debug sub-commands (queue, hook, config, raw) bind --debug-unix-socket
	// against. Without this assignment, those commands default to
	// /var/run/shell-operator/debug.socket while the running operator actually
	// listens on cfg.Debug.UnixSocket (set by the DEBUG_UNIX_SOCKET env var in
	// modules/002-deckhouse/templates/deployment.yaml). This mirrors what
	// addon-operator's own cmd/addon-operator/main.go does before
	// sh_debug.DefineDebugCommands(rootCmd) below. In the `start` path,
	// NewAddonOperator also assigns sh_debug.DefaultSocketPath, so the two
	// assignments are idempotent.
	ad_app.ApplyConfig(cfg)
	sh_debug.DefaultSocketPath = cfg.Debug.UnixSocket

	logger := log.NewLogger()
	log.SetDefault(logger)

	fileName := filepath.Base(os.Args[0])

	rootCmd := &cobra.Command{
		Use:   fileName,
		Short: fmt.Sprintf("%s %s: %s", AppName, DeckhouseVersion, AppDescription),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			klogtolog.InitAdapter(cfg.Debug.KubernetesAPI, logger.Named("klog"))
			stdliblogtolog.InitAdapter(logger)
			return nil
		},
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

	// dhctlcli command builders previously relied on dhctl/pkg/app package-level
	// globals. They now read configuration from a dedicated *options.Options;
	// the kingpin Envar bindings in dhctl/pkg/app are gated by flag registration
	// (DefineGlobalFlags / DefineKubeFlags) which we do not invoke, so we seed
	// the options struct directly from deployer-controlled env vars here.
	//
	// dhctl/cmd/dhctl/commands/{edit,config}.go remain kingpin-based and the
	// internal/dhctlcli mirror is enforced to stay in sync via
	// tools/check-dhctl-cmd-drift.sh, so we bridge those commands into the
	// cobra root via stub commands with DisableFlagParsing that delegate the
	// remaining argv to a kingpin Application built on the fly.
	{
		dhctlOpts := options.New()
		dhctlOpts.Global.LoggerType = envOr("DECKHOUSE_LOGGER_TYPE", "json")
		dhctlOpts.Render.Editor = envOr("DECKHOUSE_EDITOR", "vim")
		dhctlOpts.Kube.InCluster = envBoolOr("DECKHOUSE_KUBE_CONFIG_IN_CLUSTER", true)
		dhctlOpts.Global.TmpDir = envOr("DECKHOUSE_TMP_DIR", os.TempDir())

		// Pin the dhctl content directories to the deckhouse image layout
		// (/deckhouse/...). The legacy kingpin entrypoint relied on
		// dhctl/pkg/config package globals that defaulted to these absolute
		// paths, so commands like `cluster-configuration` and
		// `cloud-discovery-data` found their schemas under /deckhouse/candi.
		// options.New() instead calls NewGlobalOptions(), which auto-detects
		// these dirs relative to the current working directory and otherwise
		// points them at a download dir under TmpDir (/tmp/dhctl/deckhouse/...).
		// That directory does not exist in the deckhouse image, so config
		// parsing failed with "init configuration index not found". Restore the
		// previous behavior by setting the image paths explicitly.
		dhctlOpts.Global.DeckhouseDir = options.DefaultDeckhouseDir
		dhctlOpts.Global.CandiDir = options.DefaultCandiDir
		dhctlOpts.Global.ModulesDir = options.DefaultModulesDir
		dhctlOpts.Global.GlobalHooksModule = options.DefaultGlobalHooksModule
		dhctlOpts.Global.InfrastructureVersions = options.DefaultInfrastructureVersions
		dhctlOpts.Global.VersionMap = options.DefaultVersionMap
		dhctlOpts.Global.EnsureCandiAvailable = false

		dhctlcli.RegisterCobraBridges(rootCmd, fileName, dhctlOpts)
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
