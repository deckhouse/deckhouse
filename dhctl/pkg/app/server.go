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
)

var (
	ServerNetwork            string
	ServerAddress            string
	ServerParallelTasksLimit int
)

func DefineServerFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("server-network", "").
		Envar(configEnvName("SERVER_NETWORK")).
		Default("tcp").
		EnumVar(&ServerNetwork, "tcp", "unix")
	cmd.Flag("server-address", "").
		Envar(configEnvName("SERVER_ADDRESS")).
		StringVar(&ServerAddress)
	cmd.Flag("server-parallel-tasks-limit", "").
		Envar(configEnvName("SERVER_PARALLEL_TASKS_LIMIT")).
		Default("10").
		IntVar(&ServerParallelTasksLimit)
}
