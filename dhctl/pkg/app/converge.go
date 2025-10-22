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
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	MetricsPath   = "/metrics"
	ListenAddress = ":9101"
	CheckInterval = time.Minute
	OutputFormat  = "yaml"

	CheckHasTerraformStateBeforeMigrateToTofu = false
)

func DefineConvergeExporterFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("metrics-path", "Path to export metrics").
		Envar(configEnvName("METRICS_PATH")).
		StringVar(&MetricsPath)
	cmd.Flag("listen-address", "Address to expose metrics").
		Envar(configEnvName("LISTEN_ADDRESS")).
		StringVar(&ListenAddress)
	cmd.Flag("check-interval", "Period to check infrastructure state converge").
		Envar(configEnvName("CHECK_INTERVAL")).
		DurationVar(&CheckInterval)
}

func DefineOutputFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("output", "Output format").
		Envar(configEnvName("OUTPUT")).
		Short('o').
		EnumVar(&OutputFormat, "yaml", "json")
}

func DefineCheckHasTerraformStateBeforeMigrateToTofu(cmd *kingpin.CmdClause) {
	cmd.Flag("check-has-terraform-state-before-migrate-to-tofu", "Check cluster has terraform state before migrate state to tofu.").
		Default("false").
		BoolVar(&CheckHasTerraformStateBeforeMigrateToTofu)
}
