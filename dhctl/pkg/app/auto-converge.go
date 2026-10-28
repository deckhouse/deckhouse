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
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineAutoConvergeFlags registers the auto-converge service flags.
func DefineAutoConvergeFlags(cmd *cobra.Command, o *options.AutoConvergeOptions) {
	flags := cmd.Flags()
	DurationFlag(flags, Flag{
		Name: "converge-interval",
		Env:  "DHCTL_CLI_CONVERGE_INTERVAL",
		Help: "Period to converge infrastructure state",
	}, &o.ApplyInterval)

	StringFlag(flags, Flag{
		Name: "listen-address",
		Env:  "DHCTL_CLI_LISTEN_ADDRESS",
		Help: "Address to expose metrics",
	}, &o.ListenAddress)

	StringFlag(flags, Flag{
		Name: "node-name",
		Env:  "DHCTL_CLI_RUNNING_NODE_NAME",
		Help: "Name of the node where the auto-converger pod is running",
	}, &o.RunningNodeName)
}
