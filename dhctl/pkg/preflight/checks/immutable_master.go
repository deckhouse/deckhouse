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
	ImmutableSysextDigestsCheckName      preflight.CheckName = "immutable-sysext-digests"
	ImmutableControlPlaneImagesCheckName preflight.CheckName = "immutable-control-plane-images"
	ImmutableRegistryModeCheckName       preflight.CheckName = "immutable-registry-mode"
	ImmutablePostBootstrapHookCheckName  preflight.CheckName = "immutable-post-bootstrap-script"
	ImmutableSignatureModeCheckName      preflight.CheckName = "immutable-signature-mode"
	ImmutableKubeconfigOutCheckName      preflight.CheckName = "immutable-kubeconfig-out"
	ImmutableKubeconfigKeptCheckName     preflight.CheckName = "immutable-kubeconfig-kept"
)

func noRetry() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

// ImmutableSysextDigests fails early when the installer image does not carry
// the system extensions the node needs. Without them the node has nothing to
// merge onto its read-only root and never starts kubelet.
func ImmutableSysextDigests(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:        ImmutableSysextDigestsCheckName,
		Description: "installer image carries the containerd, CNI and kubelet system extensions",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(ctx context.Context) error {
			if metaConfig == nil {
				return errors.New("meta config is nil")
			}
			_, err := immutable.SysextDigests(ctx, metaConfig)
			return err
		},
	}
}

// ImmutableControlPlaneImages fails early when the installer image carries no
// control plane for the cluster's Kubernetes version: an unresolved image reaches
// the node as an empty string and the static pod never starts, silently.
func ImmutableControlPlaneImages(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:        ImmutableControlPlaneImagesCheckName,
		Description: "installer image carries the control plane of the requested Kubernetes version",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(ctx context.Context) error {
			if metaConfig == nil {
				return errors.New("meta config is nil")
			}
			_, err := immutable.ResolveControlPlaneImages(ctx, metaConfig)
			return err
		},
	}
}

// ImmutableRegistryMode rejects the registry modes an immutable master cannot
// use: it pulls from the registry directly, with no in-cluster proxy to route
// through while it is still bringing the cluster up.
func ImmutableRegistryMode(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:        ImmutableRegistryModeCheckName,
		Description: "registry runs in Unmanaged mode",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(_ context.Context) error {
			mode := metaConfig.Registry.Settings.Mode
			if mode != constant.ModeUnmanaged {
				return fmt.Errorf(
					"an immutable master supports registry mode %q only, got %q: the node pulls from the registry directly during bootstrap",
					constant.ModeUnmanaged, mode,
				)
			}
			return nil
		},
	}
}

// ImmutableSignatureMode rejects a cluster that asks kube-apiserver to verify
// object signatures: the keys and encryption config behind that are uploaded over
// SSH by a preparator the immutable path never runs, and the apiserver would crash-loop.
func ImmutableSignatureMode(metaConfig *config.MetaConfig, globalOpts *options.GlobalOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutableSignatureModeCheckName,
		Description: "control-plane signature mode is off",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(ctx context.Context) error {
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
		},
	}
}

// ImmutableKubeconfigKept rejects a --kubeconfig-out that dhctl would delete on
// its way out: the tmp cleaner empties the very directory the flag's help points
// at, and on an immutable cluster that file is the only way in.
func ImmutableKubeconfigKept(bootstrapOpts *options.BootstrapOptions, globalOpts *options.GlobalOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutableKubeconfigKeptCheckName,
		Description: "the admin kubeconfig is written somewhere dhctl will not delete",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(ctx context.Context) error {
			return immutable.CheckKubeconfigOutSurvivesCleanup(ctx, bootstrapOpts.KubeconfigOut, globalOpts.TmpDir)
		},
	}
}

// ImmutableKubeconfigOutOptions carries the one thing the check cannot read off
// the bootstrap options: whether dhctl is driven by dhctl-server (a bootstrapper
// field, recorded nowhere in options.Options).
type ImmutableKubeconfigOutOptions struct {
	CommanderMode bool
}

// ImmutableKubeconfigOut rejects a Commander-mode bootstrap that names no path for
// the admin kubeconfig: dhctl-server writes no default (TmpDir is shared by every
// cluster) and the bootstrap response carries none, so the one-shot credentials would be lost.
func ImmutableKubeconfigOut(bootstrapOpts *options.BootstrapOptions, opts ImmutableKubeconfigOutOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutableKubeconfigOutCheckName,
		Description: "the admin kubeconfig has somewhere to be written",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(_ context.Context) error {
			if !opts.CommanderMode || bootstrapOpts.KubeconfigOut != "" {
				return nil
			}
			return immutable.ErrKubeconfigOutRequired
		},
	}
}

// ImmutablePostBootstrapScript rejects --post-bootstrap-script-path: the script
// runs over SSH on the master, and an immutable node has no sshd.
func ImmutablePostBootstrapScript(bootstrapOpts *options.BootstrapOptions) preflight.Check {
	return preflight.Check{
		Name:        ImmutablePostBootstrapHookCheckName,
		Description: "no post-bootstrap script is requested",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(_ context.Context) error {
			if bootstrapOpts.PostBootstrapScriptPath == "" {
				return nil
			}
			return fmt.Errorf(
				"--post-bootstrap-script-path (%s) is not supported for an immutable master: the script is executed over SSH and an immutable node runs no sshd",
				bootstrapOpts.PostBootstrapScriptPath,
			)
		},
	}
}
