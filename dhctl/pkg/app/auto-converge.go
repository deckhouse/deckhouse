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
	ApplyInterval             = 30 * time.Minute
	AutoConvergeListenAddress = ":9101"
	RunningNodeName           = ""
)

func DefineAutoConvergeFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("converge-interval", "Period to converge infrastructure state").
		Envar(configEnvName("CONVERGE_INTERVAL")).
		DurationVar(&ApplyInterval)

	cmd.Flag("listen-address", "Address to expose metrics").
		Envar(configEnvName("LISTEN_ADDRESS")).
		StringVar(&AutoConvergeListenAddress)

	cmd.Flag("node-name", "Node name where running auto-converger pod").
		Envar(configEnvName("RUNNING_NODE_NAME")).
		StringVar(&RunningNodeName)
}
