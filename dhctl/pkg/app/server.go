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
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineServerFlags registers gRPC server flags into o.
func DefineServerFlags(cmd *kingpin.CmdClause, o *options.ServerOptions) {
	cmd.Flag("server-network", "").
		Envar(configEnvName("SERVER_NETWORK")).
		Default("tcp").
		EnumVar(&o.Network, "tcp", "unix")
	cmd.Flag("server-address", "").
		Envar(configEnvName("SERVER_ADDRESS")).
		StringVar(&o.Address)
	cmd.Flag("server-parallel-tasks-limit", "").
		Envar(configEnvName("SERVER_PARALLEL_TASKS_LIMIT")).
		Default("10").
		IntVar(&o.ParallelTasksLimit)
	cmd.Flag("server-requests-counter-max-duration", "").
		Default("2h").
		Envar(configEnvName("SERVER_REQUESTS_COUNTER_MAX_DURATION")).
		DurationVar(&o.RequestsCounterMaxDuration)
}
