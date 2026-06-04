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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineBashibleBundleFlags registers --internal-node-ip and --device-path.
func DefineBashibleBundleFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions) {
	cmd.Flag("internal-node-ip", "Address of a node from internal network.").
		Required().
		Envar(configEnvName("INTERNAL_NODE_IP")).
		StringVar(&o.InternalNodeIP)
	cmd.Flag("device-path", "Path of kubernetes-data device.").
		Required().
		Envar(configEnvName("DEVICE_PATH")).
		StringVar(&o.DevicePath)
}

// DefineDeckhouseFlags registers --deckhouse-timeout.
func DefineDeckhouseFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions) {
	cmd.Flag("deckhouse-timeout", "Timeout to install deckhouse. Experimental. This feature may be deleted in the future.").
		Envar(configEnvName("DECKHOUSE_TIMEOUT")).
		Default(o.DeckhouseTimeout.String()).
		DurationVar(&o.DeckhouseTimeout)
}

// DefinePostBootstrapScriptFlags registers post-bootstrap script flags.
func DefinePostBootstrapScriptFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions) {
	cmd.Flag("post-bootstrap-script-path", `Path to bash (or another interpreted language which installed on master node) script which will execute after bootstrap resources.
All output of the script will be logged with Info level with prefix 'Post-bootstrap script result:'.
If you want save to state cache on key 'post-bootstrap-result' you need to out result with prefix 'Result of post-bootstrap script:' in one line.
Experimental. This feature may be deleted in the future.`).
		Envar(configEnvName("POST_BOOTSTRAP_SCRIPT_PATH")).
		StringVar(&o.PostBootstrapScriptPath)

	cmd.Flag("post-bootstrap-script-timeout", "Timeout to execute after bootstrap resources script. Experimental. This feature may be deleted in the future.").
		Envar(configEnvName("POST_BOOTSTRAP_SCRIPT_TIMEOUT")).
		Default(o.PostBootstrapScriptTimeout.String()).
		DurationVar(&o.PostBootstrapScriptTimeout)
}

// DefineResourcesFlags registers --resources / --resources-timeout.
func DefineResourcesFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions, isRequired bool) {
	cmd.Flag("resources", `Path to a file with declared Kubernetes resources in YAML format.
Deprecated. Please use --config flag multiple repeatedly for logical resources separation.
`).
		Envar(configEnvName("RESOURCES")).
		StringVar(&o.ResourcesPath)
	cmd.Flag("resources-timeout", "Timeout to create resources. Experimental. This feature may be deleted in the future.").
		Envar(configEnvName("RESOURCES_TIMEOUT")).
		Default(o.ResourcesTimeout.String()).
		DurationVar(&o.ResourcesTimeout)
	if isRequired {
		cmd.GetFlag("resources").Required()
	}
}

// DefineAbortFlags registers --force-abort-from-cache.
func DefineAbortFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions) {
	const help = `Skip 'use dhctl destroy command' error. It force bootstrap abortion from cache.
Experimental. This feature may be deleted in the future.`
	cmd.Flag("force-abort-from-cache", help).
		Envar(configEnvName("FORCE_ABORT_FROM_CACHE")).
		Default("false").
		BoolVar(&o.ForceAbortFromCache)
}

// DefineDontUsePublicImagesFlags registers --dont-use-public-control-plane-images.
func DefineDontUsePublicImagesFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions) {
	const help = `DEPRECATED. Don't use public images for control-plane components.`
	cmd.Flag("dont-use-public-control-plane-images", help).
		Envar(configEnvName("DONT_USE_PUBLIC_CONTROL_PLANE_IMAGES")).
		Default("false").
		BoolVar(&o.DontUsePublicControlPlaneImages)
}

// DefineDeckhouseInstallFlags registers --kubeadm-bootstrap and --master-node-selector.
func DefineDeckhouseInstallFlags(cmd *kingpin.CmdClause, o *options.BootstrapOptions) {
	cmd.Flag("kubeadm-bootstrap", "Use default Kubernetes API server host and port for Kubeadm installations to install Deckhouse.").
		Envar(configEnvName("KUBEADM_BOOTSTRAP")).
		Default("false").
		BoolVar(&o.KubeadmBootstrap)
	cmd.Flag("master-node-selector", "Schedule Deckhouse on master nodes.").
		Envar(configEnvName("MASTER_NODE_SELECTOR")).
		Default("false").
		BoolVar(&o.MasterNodeSelector)
}

// DefineConfigsForResourcesPhaseFlags registers a non-required `--config` for the
// `bootstrap-phase create-resources` command (kept separate from DefineConfigFlags
// because that variant marks the flag as required).
func DefineConfigsForResourcesPhaseFlags(cmd *kingpin.CmdClause, o *options.GlobalOptions) {
	cmd.Flag("config", `Path to a file with bootstrap configuration and declared Kubernetes resources in YAML format.`).
		Envar(configEnvName("CONFIG")).
		StringsVar(&o.ConfigPaths)
}
