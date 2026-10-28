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
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// DefineSkipPhases registers --skip-phase and checks the paths it is given against the bootstrap
// tree. Only the path itself is checked here: the cluster type is loaded by the Preparation phase,
// which runs in the command action, so whether a phase is part of this particular run cannot be
// known yet and is checked by the walk instead.
func DefineSkipPhases(cmd *cobra.Command, o *options.BootstrapOptions) {
	desc := fmt.Sprintf("Skip a bootstrap `phase` by name (repeatable). Skippable phases: %s", strings.Join(phases.SkippablePhases(phases.BootstrapPhases()), ", "))
	StringsFlag(cmd.Flags(), Flag{Name: "skip-phase", Env: "DHCTL_CLI_SKIP_PHASES", Help: desc}, &o.SkipPhases)

	AfterApply(cmd, func(*cobra.Command) error {
		_, err := phases.ResolveSkipPhases(phases.BootstrapPhases(), o.SkipPhases)
		return err
	})
}

// DefineKubeconfigOutFlags registers --kubeconfig-out.
func DefineKubeconfigOutFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	StringFlag(cmd.Flags(), Flag{Name: "kubeconfig-out", Env: "DHCTL_CLI_KUBECONFIG_OUT", Help: `Path to write the admin kubeconfig of the bootstrapped cluster to (mode 0600).
Defaults to admin.kubeconfig in the dhctl temporary directory, beside the run's log and trace.
Only a cluster whose first master runs an immutable OS produces one: such a node runs no SSH server, so the kubeconfig
the classic bootstrap leaves on the master cannot be fetched from it afterwards. The file holds cluster-admin credentials.`}, &o.KubeconfigOut)
}

// DefineDeckhouseFlags registers --deckhouse-timeout.
func DefineDeckhouseFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	DurationFlag(cmd.Flags(), Flag{
		Name: "deckhouse-timeout",
		Env:  "DHCTL_CLI_DECKHOUSE_TIMEOUT",
		Help: "Timeout to install deckhouse. Experimental. This feature may be deleted in the future.",
	}, &o.DeckhouseTimeout)
}

// DefinePostBootstrapScriptFlags registers post-bootstrap script flags.
func DefinePostBootstrapScriptFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name: "post-bootstrap-script-path",
		Env:  "DHCTL_CLI_POST_BOOTSTRAP_SCRIPT_PATH",
		Help: `Path to a bash (or other interpreted-language) script installed on the master node, which will be executed after the bootstrap resources.
All output of the script will be logged at the Info level with the prefix 'Post-bootstrap script result:'.
If you want to save to the state cache under the key 'post-bootstrap-result', you need to output the result with the prefix 'Result of post-bootstrap script:' on one line.
Experimental. This feature may be deleted in the future.`,
	}, &o.PostBootstrapScriptPath)

	DurationFlag(flags, Flag{
		Name: "post-bootstrap-script-timeout",
		Env:  "DHCTL_CLI_POST_BOOTSTRAP_SCRIPT_TIMEOUT",
		Help: "Timeout for executing the after-bootstrap-resources script. Experimental. This feature may be deleted in the future.",
	}, &o.PostBootstrapScriptTimeout)
}

// DefineResourcesFlags registers --resources / --resources-timeout.
func DefineResourcesFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name: "resources",
		Env:  "DHCTL_CLI_RESOURCES",
		Help: `Path to a file with declared Kubernetes resources in YAML format.
Deprecated. Please use the --config flag repeatedly for logical separation of resources.
`,
	}, &o.ResourcesPath)
	DurationFlag(flags, Flag{
		Name: "resources-timeout",
		Env:  "DHCTL_CLI_RESOURCES_TIMEOUT",
		Help: "Timeout to create resources. Experimental. This feature may be deleted in the future.",
	}, &o.ResourcesTimeout)
}

// DefineAbortFlags registers --force-abort-from-cache. The flag is a no-op: abort always works
// from the cache now. It stays registered because dhctl-e2e passes it on every abort invocation,
// and an unknown flag is a parse error, not a warning.
func DefineAbortFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	const help = `DEPRECATED. Has no effect: abort always works from the cache.`
	BoolFlag(cmd.Flags(), Flag{
		Name: "force-abort-from-cache",
		Env:  "DHCTL_CLI_FORCE_ABORT_FROM_CACHE",
		Help: help,
	}, &o.ForceAbortFromCache)
}

// DefineDontUsePublicImagesFlags registers --dont-use-public-control-plane-images.
func DefineDontUsePublicImagesFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	const help = `DEPRECATED. Don't use public images for control-plane components.`
	BoolFlag(cmd.Flags(), Flag{
		Name: "dont-use-public-control-plane-images",
		Env:  "DHCTL_CLI_DONT_USE_PUBLIC_CONTROL_PLANE_IMAGES",
		Help: help,
	}, &o.DontUsePublicControlPlaneImages)
}

// DefineDeckhouseInstallFlags registers --kubeadm-bootstrap and --master-node-selector.
func DefineDeckhouseInstallFlags(cmd *cobra.Command, o *options.BootstrapOptions) {
	flags := cmd.Flags()
	BoolFlag(flags, Flag{
		Name: "kubeadm-bootstrap",
		Env:  "DHCTL_CLI_KUBEADM_BOOTSTRAP",
		Help: "Use default Kubernetes API server host and port for Kubeadm installations to install Deckhouse.",
	}, &o.KubeadmBootstrap)
	BoolFlag(flags, Flag{
		Name: "master-node-selector",
		Env:  "DHCTL_CLI_MASTER_NODE_SELECTOR",
		Help: "Schedule Deckhouse on master nodes.",
	}, &o.MasterNodeSelector)
}

// DefineConfigsForResourcesPhaseFlags registers a non-required `--config` for the
// `bootstrap-phase create-resources` command (kept separate from DefineConfigFlags
// because that variant marks the flag as required).
func DefineConfigsForResourcesPhaseFlags(cmd *cobra.Command, o *options.GlobalOptions) {
	StringsFlag(cmd.Flags(), Flag{
		Name: "config",
		Env:  "DHCTL_CLI_CONFIG",
		Help: `Path to a file with bootstrap configuration and declared Kubernetes resources in YAML format.`,
	}, &o.ConfigPaths)
}
