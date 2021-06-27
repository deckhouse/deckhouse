// Copyright 2021 Flant CJSC
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

import "gopkg.in/alecthomas/kingpin.v2"

var (
	InternalNodeIP = ""
	DevicePath     = ""

	ResourcesPath = ""
)

func DefineBashibleBundleFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("internal-node-ip", "Address of a node from internal network.").
		Required().
		Envar(configEnvName("INTERNAL_NODE_IP")).
		StringVar(&InternalNodeIP)
	cmd.Flag("device-path", "Path of kubernetes-data device.").
		Required().
		Envar(configEnvName("DEVICE_PATH")).
		StringVar(&DevicePath)
}

func DefineResourcesFlags(cmd *kingpin.CmdClause, isRequired bool) {
	cmd.Flag("resources", "Path to a file with declared Kubernetes resources in YAML format.").
		Envar(configEnvName("RESOURCES")).
		StringVar(&ResourcesPath)

	if isRequired {
		cmd.GetFlag("resources").Required()
	}
}
