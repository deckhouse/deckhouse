// Copyright 2024 Flant JSC
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

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineServerFlags registers gRPC server flags into o.
func DefineServerFlags(cmd *cobra.Command, o *options.ServerOptions) {
	flags := cmd.Flags()
	o.Network = "tcp"
	EnumFlag(flags, Flag{
		Name: "server-network",
		Env:  "DHCTL_CLI_SERVER_NETWORK",
	}, &o.Network, "tcp", "unix")
	StringFlag(flags, Flag{
		Name: "server-address",
		Env:  "DHCTL_CLI_SERVER_ADDRESS",
	}, &o.Address)
	o.ParallelTasksLimit = 10
	IntFlag(flags, Flag{
		Name: "server-parallel-tasks-limit",
		Env:  "DHCTL_CLI_SERVER_PARALLEL_TASKS_LIMIT",
	}, &o.ParallelTasksLimit)
	o.RequestsCounterMaxDuration = 2 * time.Hour
	DurationFlag(flags, Flag{
		Name: "server-requests-counter-max-duration",
		Env:  "DHCTL_CLI_SERVER_REQUESTS_COUNTER_MAX_DURATION",
	}, &o.RequestsCounterMaxDuration)
}
