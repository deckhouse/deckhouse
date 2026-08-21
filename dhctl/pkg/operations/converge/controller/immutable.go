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

package controller

import (
	"fmt"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	dhctlkube "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
)

// A joining master has the whole install ahead of it — extensions, reboot, kubelet —
// and only at the end of it does control-plane-manager add its etcd member. The
// budget matches the bootstrap's wait for the same event.
const (
	joinedControlPlaneAttempts = 360
	joinedControlPlaneInterval = 5 * time.Second
)

// isImmutableNodeGroup reads systemType off the live NodeGroup. The cluster object is
// the source of truth: the bashible bootstrap secret exists for an immutable group too.
func isImmutableNodeGroup(ctx *context.Context, nodeGroupName string) (bool, error) {
	kubeCl, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return false, fmt.Errorf("get kube client to read NodeGroup %s: %w", nodeGroupName, err)
	}

	ng, err := entity.GetNodeGroupDirect(ctx.Ctx(), kubeCl, nodeGroupName)
	if err != nil {
		return false, fmt.Errorf("read NodeGroup %s to tell a mutable group from an immutable one: %w", nodeGroupName, err)
	}

	return immutable.NodeGroupIsImmutable(ng), nil
}

// immutableMasterPayload renders the cloud-init this master joins with. Unlike the
// group-wide bashible secret it is per node, so it is built where the node is created.
func immutableMasterPayload(ctx *context.Context, nodeName string) (string, error) {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return "", err
	}

	kubeCl, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return "", fmt.Errorf("get kube client to build the payload of %s: %w", nodeName, err)
	}

	// No customization and no node IP: in a cloud the machine does not exist yet, and
	// the operator describes it through the provider configuration, not per machine.
	payload, _, err := immutable.BuildJoinPayloadFromCluster(ctx.Ctx(), kubeCl, metaConfig, nodeName, nil, "")
	if err != nil {
		return "", fmt.Errorf("build the join payload of %s: %w", nodeName, err)
	}

	return payload, nil
}

// waitForImmutableMasterControlPlane waits until control-plane-manager reports this
// master ready, etcd member included. Masters join one at a time: etcd admits a single
// learner, and a second machine started meanwhile would find no room.
func waitForImmutableMasterControlPlane(ctx *context.Context, nodeName string) error {
	kubeCl, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("get kube client to wait for the control plane of %s: %w", nodeName, err)
	}

	checker := controlplane.NewManagerReadinessChecker(dhctlkube.NewSimpleKubeClientGetter(kubeCl))

	return retry.NewLoop(fmt.Sprintf("Waiting for the control plane of %s", nodeName),
		joinedControlPlaneAttempts, joinedControlPlaneInterval).
		RunContext(ctx.Ctx(), func() error {
			ready, err := checker.IsReady(ctx.Ctx(), nodeName)
			if err != nil {
				return fmt.Errorf("check the control plane of %s: %w", nodeName, err)
			}
			if !ready {
				return fmt.Errorf("the control plane of %s is not ready yet", nodeName)
			}
			return nil
		})
}
