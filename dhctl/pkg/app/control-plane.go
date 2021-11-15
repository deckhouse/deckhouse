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
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	ControlPlaneHostname = ""
	ControlPlaneIP       = ""
)

func DefineControlPlaneFlags(cmd *kingpin.CmdClause, ipRequired bool) {
	cmd.Flag("control-plane-node-hostname", "Control plane node hostname to check").
		Envar(configEnvName("CONTROL_PLANE_NODE_HOSTNAME")).
		Required().
		StringVar(&ControlPlaneHostname)

	ipFlag := cmd.Flag("control-plane-node-ip", "Control plane node ip to check").
		Envar(configEnvName("CONTROL_PLANE_NODE_IP"))

	if ipRequired {
		ipFlag.Required()
	}

	ipFlag.StringVar(&ControlPlaneIP)
}
