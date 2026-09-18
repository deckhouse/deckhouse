// Copyright 2026 Flant JSC
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

package checks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/module/controlplane"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// The checks below only run when the master NodeGroup asks for
// systemType: Immutable. Each of them guards an assumption the immutable
// bootstrap path makes that the classic bashible path does not.
const (
	ImmutableInstallerImagesCheckName      preflight.CheckName = "immutable-installer-images"
	ImmutableRegistryModeCheckName         preflight.CheckName = "immutable-registry-mode"
	ImmutablePostBootstrapScriptCheckName  preflight.CheckName = "immutable-post-bootstrap-script"
	ImmutableSignatureModeCheckName        preflight.CheckName = "immutable-signature-mode"
	ImmutableKubeconfigOutCheckName        preflight.CheckName = "immutable-kubeconfig-out"
	ImmutableKubeconfigKeptCheckName       preflight.CheckName = "immutable-kubeconfig-kept"
	ImmutableSupportedProviderCheckName    preflight.CheckName = "immutable-supported-provider"
	ImmutableMachinesAvailabilityCheckName preflight.CheckName = "immutable-machines-availability"
)

// Lowercase because MetaConfig.prepareProviderName lowercases cloud.provider;
// the config spells it "DVP" (cloudProviderToProviderKind in pkg/config/validation.go).
const immutableProviderDVP = "dvp"

// ImmutableInstallerImages fails early when the installer image does not carry
// what the node boots on: without the system extensions it never starts kubelet,
// and an unresolved control-plane image reaches it as an empty string that leaves
// the static pod silently missing.
func ImmutableInstallerImages(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:        ImmutableInstallerImagesCheckName,
		Retry:       preflight.NetworkRetry,
		Description: "installer image carries the system extensions and the control plane of the requested Kubernetes version",
		Phase:       preflight.PhasePreInfra,
		Run: preflight.Detailless(func(ctx context.Context) error {
			if metaConfig == nil {
				return errors.New("meta config is nil")
			}
			if err := immutable.ValidateSysext(ctx, metaConfig); err != nil {
				return err
			}
			_, err := immutable.ResolveControlPlaneImages(ctx, metaConfig)
			return err
		}),
	}
}

// ImmutableRegistryMode rejects the registry modes an immutable master cannot
// use: it pulls from the registry directly, with no in-cluster proxy to route
// through while it is still bringing the cluster up.
//
// Not skippable: this is not a check that might be wrong about the cluster, it is the shape of
// the bootstrap dhctl is about to run. Skipping it does not let the run proceed — it lets it
// proceed into a state the immutable path has no code for.
func ImmutableRegistryMode(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:            ImmutableRegistryModeCheckName,
		Retry:           preflight.NoRetry,
		Cacheable:       true,
		CannotBeSkipped: true,
		Description:     "registry runs in Unmanaged mode",
		Phase:           preflight.PhasePreInfra,
		Run: preflight.Detailless(func(_ context.Context) error {
			mode := metaConfig.Registry.Settings.Mode
			if mode != constant.ModeUnmanaged {
				return fmt.Errorf(
					"an immutable master supports registry mode %q only, got %q: the node pulls from the registry directly during bootstrap",
					constant.ModeUnmanaged, mode,
				)
			}
			return nil
		}),
	}
}

// ImmutableSignatureMode rejects a cluster that asks kube-apiserver to verify
// object signatures: the keys and encryption config behind that are uploaded over
// SSH by a preparator the immutable path never runs, and the apiserver would crash-loop.
func ImmutableSignatureMode(metaConfig *config.MetaConfig, globalOpts *options.GlobalOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutableSignatureModeCheckName,
		Retry:       preflight.NoRetry,
		Cacheable:   true,
		Description: "control-plane signature mode is off",
		Phase:       preflight.PhasePreInfra,
		Run: preflight.Detailless(func(ctx context.Context) error {
			extractor := controlplane.NewSettingsExtractor(
				metaConfig,
				config.NewSchemaStore(globalOpts),
				config.GetEdition(),
				dhlog.FromContext(ctx),
			)

			mode, err := extractor.SignatureMode()
			if err != nil {
				return err
			}
			if mode == controlplane.NoSignatureMode {
				return nil
			}

			return fmt.Errorf(
				"control-plane-manager runs with apiserver.signature %q, which an immutable master does not support: "+
					"the signing keys and the encryption provider config are uploaded to the node over SSH, and an immutable node runs no sshd",
				mode,
			)
		}),
	}
}

// ImmutableKubeconfigKept rejects a --kubeconfig-out that dhctl would delete on
// its way out: the tmp cleaner empties the very directory the flag's help points
// at, and on an immutable cluster that file is the only way in.
func ImmutableKubeconfigKept(bootstrapOpts *options.BootstrapOptions, globalOpts *options.GlobalOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutableKubeconfigKeptCheckName,
		Retry:       preflight.NoRetry,
		Description: "the admin kubeconfig is written somewhere dhctl will not delete",
		Phase:       preflight.PhasePreInfra,
		Run: preflight.Detailless(func(ctx context.Context) error {
			return immutable.CheckKubeconfigOutSurvivesCleanup(ctx, bootstrapOpts.KubeconfigOut, globalOpts.TmpDir)
		}),
	}
}

// ImmutableSupportedProvider keeps an immutable master on the platforms it has
// actually been tried on. The limit is temporary and lifts once the remaining
// clouds are tested.
//
// Not skippable, for the same reason as ImmutableRegistryMode: the answer is about what dhctl
// supports, not about the operator's cluster, and there is nothing behind the flag to reach.
func ImmutableSupportedProvider(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:            ImmutableSupportedProviderCheckName,
		CannotBeSkipped: true,
		Retry:           preflight.NoRetry,
		Cacheable:       true,
		Description:     "the platform is one an immutable master has been tested on",
		Phase:           preflight.PhasePreInfra,
		Run: preflight.Detailless(func(_ context.Context) error {
			if metaConfig.ClusterType == config.StaticClusterType {
				return nil
			}
			if strings.EqualFold(metaConfig.ProviderName, immutableProviderDVP) {
				return nil
			}

			return fmt.Errorf(
				"bootstrap an immutable master on cloud provider %q: immutable nodes are tested on the DVP cloud "+
					"and in a %s cluster only, and the limit lifts once the remaining clouds are tested as well",
				metaConfig.ProviderName, config.StaticClusterType,
			)
		}),
	}
}

// ImmutableKubeconfigOut rejects a Commander-mode bootstrap that names no path for
// the admin kubeconfig: dhctl-server writes no default (TmpDir is shared by every
// cluster) and the bootstrap response carries none, so the one-shot credentials would be lost.
// commanderMode is a bootstrapper field, recorded nowhere in options.Options.
func ImmutableKubeconfigOut(bootstrapOpts *options.BootstrapOptions, commanderMode bool) preflight.Check {
	return preflight.Check{
		Name:        ImmutableKubeconfigOutCheckName,
		Retry:       preflight.NoRetry,
		Description: "the admin kubeconfig has somewhere to be written",
		Phase:       preflight.PhasePreInfra,
		Run: preflight.Detailless(func(_ context.Context) error {
			if !commanderMode || bootstrapOpts.KubeconfigOut != "" {
				return nil
			}
			return immutable.ErrKubeconfigOutRequired
		}),
	}
}

// ImmutablePostBootstrapScript rejects --post-bootstrap-script-path: the script
// runs over SSH on the master, and an immutable node has no sshd.
func ImmutablePostBootstrapScript(bootstrapOpts *options.BootstrapOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutablePostBootstrapScriptCheckName,
		Retry:       preflight.NoRetry,
		Description: "no post-bootstrap script is requested",
		Phase:       preflight.PhasePreInfra,
		Run: preflight.Detailless(func(_ context.Context) error {
			if bootstrapOpts.PostBootstrapScriptPath == "" {
				return nil
			}
			return fmt.Errorf(
				"--post-bootstrap-script-path (%s) is not supported for an immutable master: the script is executed over SSH and an immutable node runs no sshd",
				bootstrapOpts.PostBootstrapScriptPath,
			)
		}),
	}
}

// ImmutableMachinesAvailability asks the machines themselves: each one named with
// --master-host answers its maintenance port, and the hardware it reports is
// the hardware its document describes. Both come from one inventory read.
//
// The work lives in the bootstrapper — it owns the tunnel and the documents — and arrives here
// as run, which is also where the "no machines were named" verdict is decided, because that is
// where the host list is. The nil guard below is for a suite built without one at all.
func ImmutableMachinesAvailability(run func(context.Context) error) preflight.Check {
	return preflight.Check{
		Name:        ImmutableMachinesAvailabilityCheckName,
		Retry:       preflight.NoRetry,
		Timeout:     preflight.LongCheckTimeout,
		Description: "the machines named with --master-host answer and match the configuration written for them",
		Phase:       preflight.PhasePreInfra,
		Run: preflight.Detailless(func(ctx context.Context) error {
			if run == nil {
				return preflight.NotApplicable("this bootstrap names no machines with --master-host; the provider creates the masters")
			}
			return run(ctx)
		}),
	}
}
