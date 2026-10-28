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

package app

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineConvergeExporterFlags registers the converge-exporter metrics flags.
func DefineConvergeExporterFlags(cmd *cobra.Command, o *options.ConvergeOptions) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name: "metrics-path",
		Env:  "DHCTL_CLI_METRICS_PATH",
		Help: "Path to export metrics",
	}, &o.MetricsPath)
	StringFlag(flags, Flag{
		Name: "listen-address",
		Env:  "DHCTL_CLI_LISTEN_ADDRESS",
		Help: "Address to expose metrics",
	}, &o.ListenAddress)
	DurationFlag(flags, Flag{
		Name: "check-interval",
		Env:  "DHCTL_CLI_CHECK_INTERVAL",
		Help: "Period for checking infrastructure state convergence",
	}, &o.CheckInterval)
}

func DefineConvergeFlags(cmd *cobra.Command, o *options.ConvergeOptions) {
	BoolFlag(cmd.Flags(), Flag{
		Name: "converge-destructive-auto-approve",
		Env:  "DHCTL_CLI_DESTRUCTIVE_APPROVED",
		Help: "Destructive changes are auto-approved, only for test and dev purposes, use carefully!",
	}, &o.DestructiveApproved)
}

// DefineOutputFlag registers --output / -o for the check-style commands.
func DefineOutputFlag(cmd *cobra.Command, o *options.ConvergeOptions) {
	EnumFlag(cmd.Flags(), Flag{
		Name:  "output",
		Short: "o",
		Env:   "DHCTL_CLI_OUTPUT",
		Help:  "Output format",
	}, &o.OutputFormat, "yaml", "json")
}

// DefineCheckHasTerraformStateBeforeMigrateToTofu registers the migration guard flag.
func DefineCheckHasTerraformStateBeforeMigrateToTofu(cmd *cobra.Command, o *options.ConvergeOptions) {
	BoolFlag(cmd.Flags(), Flag{
		Name: "check-has-terraform-state-before-migrate-to-tofu",
		Help: "Check that the cluster has terraform state before migrating the state to tofu.",
	}, &o.CheckHasTerraformStateBeforeMigrateToTofu)
}

// ForceNoSwitchToNodeUser reports whether DHCTL_CLI_NO_SWITCH_TO_NODE_USER=true.
// It reads the environment directly and does not depend on parsed options.
func ForceNoSwitchToNodeUser() bool {
	return os.Getenv("DHCTL_CLI_NO_SWITCH_TO_NODE_USER") == "true"
}

// SkipDrainingNodes reports whether DHCTL_CLI_SKIP_DRAINING_NO_NODES=true.
// It reads the environment directly and does not depend on parsed options.
func SkipDrainingNodes() bool {
	return os.Getenv("DHCTL_CLI_SKIP_DRAINING_NO_NODES") == "true"
}
