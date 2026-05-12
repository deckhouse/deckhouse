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

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineConvergeExporterFlags registers the converge-exporter metrics flags.
func DefineConvergeExporterFlags(cmd *kingpin.CmdClause, o *options.ConvergeOptions) {
	cmd.Flag("metrics-path", "Path to export metrics").
		Envar(configEnvName("METRICS_PATH")).
		StringVar(&o.MetricsPath)
	cmd.Flag("listen-address", "Address to expose metrics").
		Envar(configEnvName("LISTEN_ADDRESS")).
		StringVar(&o.ListenAddress)
	cmd.Flag("check-interval", "Period to check infrastructure state converge").
		Envar(configEnvName("CHECK_INTERVAL")).
		DurationVar(&o.CheckInterval)
}

// DefineOutputFlag registers --output / -o for the check-style commands.
func DefineOutputFlag(cmd *kingpin.CmdClause, o *options.ConvergeOptions) {
	cmd.Flag("output", "Output format").
		Envar(configEnvName("OUTPUT")).
		Short('o').
		EnumVar(&o.OutputFormat, "yaml", "json")
}

// DefineCheckHasTerraformStateBeforeMigrateToTofu registers the migration guard flag.
func DefineCheckHasTerraformStateBeforeMigrateToTofu(cmd *kingpin.CmdClause, o *options.ConvergeOptions) {
	cmd.Flag("check-has-terraform-state-before-migrate-to-tofu", "Check cluster has terraform state before migrate state to tofu.").
		Default("false").
		BoolVar(&o.CheckHasTerraformStateBeforeMigrateToTofu)
}

// ForceNoSwitchToNodeUser reports whether DHCTL_CLI_NO_SWITCH_TO_NODE_USER=true.
// It reads the environment directly and does not depend on parsed options.
func ForceNoSwitchToNodeUser() bool {
	return getEnvBool("NO_SWITCH_TO_NODE_USER")
}

// SkipDrainingNodes reports whether DHCTL_CLI_SKIP_DRAINING_NO_NODES=true.
// It reads the environment directly and does not depend on parsed options.
func SkipDrainingNodes() bool {
	return getEnvBool("SKIP_DRAINING_NO_NODES")
}

func getEnvBool(name string) bool {
	envName := configEnvName(name)
	if val, ok := os.LookupEnv(envName); ok && val == "true" {
		return true
	}
	return false
}
