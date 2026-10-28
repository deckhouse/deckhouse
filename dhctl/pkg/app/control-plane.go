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

// DefineControlPlaneFlags registers the control-plane test-target flags into o.
func DefineControlPlaneFlags(cmd *cobra.Command, o *options.ControlPlaneOptions, ipRequired bool) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name:     "control-plane-node-hostname",
		Env:      "DHCTL_CLI_CONTROL_PLANE_NODE_HOSTNAME",
		Help:     "Control plane node hostname to check",
		Required: true,
	}, &o.Hostname)

	StringFlag(flags, Flag{
		Name:     "control-plane-node-ip",
		Env:      "DHCTL_CLI_CONTROL_PLANE_NODE_IP",
		Help:     "Control plane node IP to check",
		Required: ipRequired,
	}, &o.IP)
}
