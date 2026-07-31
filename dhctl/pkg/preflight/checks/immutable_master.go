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
	"fmt"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// The checks below only run when the master NodeGroup asks for
// systemType: Immutable. Each of them guards an assumption the immutable
// bootstrap path makes that the classic bashible path does not.
const (
	ImmutableSysextDigestsCheckName     preflight.CheckName = "immutable-sysext-digests"
	ImmutableRegistryModeCheckName      preflight.CheckName = "immutable-registry-mode"
	ImmutableMasterDisksCheckName       preflight.CheckName = "immutable-master-disks"
	ImmutableMasterReplicasCheckName    preflight.CheckName = "immutable-master-replicas"
	ImmutablePostBootstrapHookCheckName preflight.CheckName = "immutable-post-bootstrap-script"
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
		Run: func(_ context.Context) error {
			if metaConfig == nil {
				return fmt.Errorf("meta config is nil")
			}
			_, err := immutable.SysextDigests(metaConfig)
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
			if metaConfig == nil {
				return fmt.Errorf("meta config is nil")
			}
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

// ImmutableMasterDisks validates the master's disk configuration. dhctl builds
// the payload before the VM exists, so it can only tell the root disk from the
// etcd disk by size.
func ImmutableMasterDisks(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:        ImmutableMasterDisksCheckName,
		Description: "master declares an etcd disk smaller than its root disk",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(_ context.Context) error {
			if metaConfig == nil {
				return fmt.Errorf("meta config is nil")
			}
			_, _, err := immutable.MasterDisks(metaConfig)
			return err
		},
	}
}

// ImmutableMasterReplicas rejects a multi-master immutable group. The extra
// masters are ordered with the cloud config node-manager publishes for the
// group, which is a bashible bundle an immutable node cannot run.
func ImmutableMasterReplicas(metaConfig *config.MetaConfig) preflight.Check {
	return preflight.Check{
		Name:        ImmutableMasterReplicasCheckName,
		Description: "immutable master group has a single replica",
		Phase:       preflight.PhasePreInfra,
		Retry:       noRetry(),
		Run: func(_ context.Context) error {
			if metaConfig == nil {
				return fmt.Errorf("meta config is nil")
			}
			if replicas := metaConfig.MasterNodeGroupSpec.Replicas; replicas > 1 {
				return fmt.Errorf("masterNodeGroup.replicas is %d: an immutable master group supports a single replica for now", replicas)
			}
			return nil
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
			if bootstrapOpts == nil || bootstrapOpts.PostBootstrapScriptPath == "" {
				return nil
			}
			return fmt.Errorf(
				"--post-bootstrap-script-path (%s) is not supported for an immutable master: the script is executed over SSH and an immutable node runs no sshd",
				bootstrapOpts.PostBootstrapScriptPath,
			)
		},
	}
}
