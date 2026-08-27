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
	gocontext "context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	dhctlkube "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/registrydata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
)

// Same budget as entity.GetNodeGroup: a NodeGroup read must survive an apiserver
// restart, and every converged group starts with one.
const (
	nodeGroupReadAttempts = 600
	nodeGroupReadInterval = 1 * time.Second
)

// isImmutableNodeGroup reads systemType off the live NodeGroup. The cluster object is
// the source of truth: the bashible bootstrap secret exists for an immutable group too.
func isImmutableNodeGroup(ctx *context.Context, nodeGroupName string) (bool, error) {
	kubeCl, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return false, fmt.Errorf("get kube client to read NodeGroup %s: %w", nodeGroupName, err)
	}

	var ng *unstructured.Unstructured
	err = retry.NewSilentLoop(fmt.Sprintf("Get NodeGroup %q", nodeGroupName), nodeGroupReadAttempts, nodeGroupReadInterval).
		BreakIf(apierrors.IsNotFound).
		RunContext(ctx.Ctx(), func() error {
			var err error
			ng, err = entity.GetNodeGroupDirect(ctx.Ctx(), kubeCl, nodeGroupName)

			return err
		})

	// A NodeGroup deleted from the cluster can still hold infrastructure state, and
	// converge exists to delete those machines. It needs no systemType to do it.
	if apierrors.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("read NodeGroup %s to tell a mutable group from an immutable one: %w", nodeGroupName, err)
	}

	return immutable.NodeGroupIsImmutable(ng), nil
}

// immutableNodePayload renders the cloud-init this node joins with. Unlike the
// group-wide bashible secret it is per node, so it is built where the node is created.
func immutableNodePayload(ctx *context.Context, nodeGroupName, nodeName string) (string, error) {
	shared, err := ctx.MetaConfig()
	if err != nil {
		return "", err
	}

	// The digests and the cluster registry below are edits for this one render. The
	// context hands out the same MetaConfig to everything else in the converge run,
	// and in AutoConverge the run outlives the tick.
	metaConfig := shared.DeepCopy()

	// A config read from the cluster carries no image digests — only the bootstrap's
	// file path loads them — and the payload names every extension by digest. They come
	// from the installer image, so loading them here reads no cluster state.
	if len(metaConfig.Images) == 0 {
		if err := metaConfig.LoadImagesDigests(); err != nil {
			return "", fmt.Errorf("load the image digests the payload of %s names: %w", nodeName, err)
		}
	}

	kubeCl, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return "", fmt.Errorf("get kube client to build the payload of %s: %w", nodeName, err)
	}

	if err := useClusterRegistry(ctx.Ctx(), kubeCl, metaConfig); err != nil {
		return "", fmt.Errorf("read the registry the payload of %s pulls from: %w", nodeName, err)
	}

	// No customization and no node IP: in a cloud the machine does not exist yet, and
	// the operator describes it through the provider configuration, not per machine.
	payload, _, err := immutable.BuildJoinPayloadFromCluster(ctx.Ctx(), kubeCl, metaConfig, nodeName, nil, "", nodeGroupName)
	if err != nil {
		return "", fmt.Errorf("build the join payload of %s: %w", nodeName, err)
	}

	return payload, nil
}

// NewImmutablePayloadBuilder hands immutableNodePayload to the bootstrap helpers, which
// create the machines of a group that has no infrastructure state yet. They call it only
// for a group whose systemType is Immutable.
func NewImmutablePayloadBuilder(ctx *context.Context) operations.ImmutablePayloadBuilder {
	return func(_ gocontext.Context, _ *client.KubernetesClient, nodeGroupName, nodeName string) (string, error) {
		return immutableNodePayload(ctx, nodeGroupName, nodeName)
	}
}

// useClusterRegistry points the payload at the registry the cluster itself pulls from.
// A configuration parsed from the cluster carries no registry credentials — with no
// registry ModuleConfig it resolves to the CE default — while the node has to reach the
// very registry the running cluster uses. The upstream address is preferred over the
// in-cluster mirror: a machine that is still installing cannot resolve a Service.
func useClusterRegistry(ctx gocontext.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) error {
	conf, _, err := registrydata.GetRegistryDataPreferUpstream(ctx, kubeCl, false)
	if err != nil {
		return err
	}

	imagesRepo := conf.GetRegistry()
	if imagesRepo == "" {
		return errors.New("the cluster publishes no registry address")
	}

	metaConfig.Registry.Settings.RemoteData = registry.Data{
		ImagesRepo: imagesRepo,
		Scheme:     constant.SchemeType(conf.GetScheme()),
		CA:         conf.GetCA(),
		Username:   conf.GetUsername(),
		Password:   conf.GetPassword(),
	}

	return nil
}

// waitForImmutableMasterControlPlane waits until control-plane-manager reports this
// master ready, etcd member included. Masters join one at a time: etcd admits a single
// learner, and a second machine started meanwhile would find no room.
func waitForImmutableMasterControlPlane(ctx *context.Context, nodeName string) error {
	kubeCl, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("get kube client to wait for the control plane of %s: %w", nodeName, err)
	}

	return controlplane.NewManagerReadinessChecker(dhctlkube.NewSimpleKubeClientGetter(kubeCl)).WaitReady(ctx.Ctx(), nodeName)
}
