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
	InternalNodeIP = ""
	DevicePath     = ""

	ResourcesPath    = ""
	ResourcesTimeout = 15 * time.Minute
	DeckhouseTimeout = 15 * time.Minute

	PostBootstrapScriptTimeout = 10 * time.Minute
	PostBootstrapScriptPath    = ""

	ForceAbortFromCache             = false
	DontUsePublicControlPlaneImages = false

	KubeadmBootstrap   = false
	MasterNodeSelector = false
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

func DefineDeckhouseFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("deckhouse-timeout", "Timeout to install deckhouse. Experimental. This feature may be deleted in the future.").
		Envar(configEnvName("DECKHOUSE_TIMEOUT")).
		Default(DeckhouseTimeout.String()).
		DurationVar(&DeckhouseTimeout)
}

func DefinePostBootstrapScriptFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("post-bootstrap-script-path", `Path to bash (or another interpreted language which installed on master node) script which will execute after bootstrap resources.
All output of the script will be logged with Info level with prefix 'Post-bootstrap script result:'.
If you want save to state cache on key 'post-bootstrap-result' you need to out result with prefix 'Result of post-bootstrap script:' in one line.
Experimental. This feature may be deleted in the future.`).
		Envar(configEnvName("POST_BOOTSTRAP_SCRIPT_PATH")).
		StringVar(&PostBootstrapScriptPath)

	cmd.Flag("post-bootstrap-script-timeout", "Timeout to execute after bootstrap resources script. Experimental. This feature may be deleted in the future.").
		Envar(configEnvName("POST_BOOTSTRAP_SCRIPT_TIMEOUT")).
		Default(PostBootstrapScriptTimeout.String()).
		DurationVar(&PostBootstrapScriptTimeout)
}

func DefineResourcesFlags(cmd *kingpin.CmdClause, isRequired bool) {
	cmd.Flag("resources", `Path to a file with declared Kubernetes resources in YAML format. It can be go-template file. Passed data contains next keys:
  cloudDiscovery - the data discovered by applying Terrfarorm and getting its output. It depends on the cloud provider.
`).
		Envar(configEnvName("RESOURCES")).
		StringVar(&ResourcesPath)
	cmd.Flag("resources-timeout", "Timeout to create resources. Experimental. This feature may be deleted in the future.").
		Envar(configEnvName("RESOURCES_TIMEOUT")).
		Default(ResourcesTimeout.String()).
		DurationVar(&ResourcesTimeout)
	if isRequired {
		cmd.GetFlag("resources").Required()
	}
}

func DefineAbortFlags(cmd *kingpin.CmdClause) {
	const help = `Skip 'use dhctl destroy command' error. It force bootstrap abortion from cache.
Experimental. This feature may be deleted in the future.`
	cmd.Flag("force-abort-from-cache", help).
		Envar(configEnvName("FORCE_ABORT_FROM_CACHE")).
		Default("false").
		BoolVar(&ForceAbortFromCache)
}

func DefineDontUsePublicImagesFlags(cmd *kingpin.CmdClause) {
	const help = `DEPRECATED. Don't use public images for control-plane components.`
	cmd.Flag("dont-use-public-control-plane-images", help).
		Envar(configEnvName("DONT_USE_PUBLIC_CONTROL_PLANE_IMAGES")).
		Default("false").
		BoolVar(&DontUsePublicControlPlaneImages)
}

func DefineDeckhouseInstallFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("kubeadm-bootstrap", "Use default Kubernetes API server host and port for Kubeadm installations to install Deckhouse.").
		Envar(configEnvName("KUBEADM_BOOTSTRAP")).
		Default("false").
		BoolVar(&KubeadmBootstrap)
	cmd.Flag("master-node-selector", "Schedule Deckhouse on master nodes.").
		Envar(configEnvName("MASTER_NODE_SELECTOR")).
		Default("false").
		BoolVar(&MasterNodeSelector)
}
