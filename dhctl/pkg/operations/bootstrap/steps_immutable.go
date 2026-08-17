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

package bootstrap

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
)

// immutableBootstrap is what this path carries between phases. A node on it
// runs no sshd and no bashible: dhctl hands it a cloud-init payload and then
// only talks to its API, over a bastion tunnel when there is one.
type immutableBootstrap struct {
	masterNodeName string
	// masterIP is where that API answers; the BaseInfra phase reports it.
	masterIP string
	// kubeconfigPath is where the collected admin credentials were written, empty
	// until the node has handed them over.
	kubeconfigPath string
	tunnelStop     func()
}

// stopImmutableTunnel closes the bastion tunnel the path opened, if it opened one.
func (c *bootstrapContext) stopImmutableTunnel() {
	if c.immutable == nil || c.immutable.tunnelStop == nil {
		return
	}
	c.immutable.tunnelStop()
}

// printCollectedKubeconfig says how to reach the cluster once the node has
// handed its credentials over. Silent until then, and on every other path.
func (b *ClusterBootstrapper) printCollectedKubeconfig(ctx context.Context, bctx *bootstrapContext) {
	if bctx.immutable == nil || bctx.immutable.kubeconfigPath == "" {
		return
	}
	b.printHowToReachTheCluster(ctx, bctx.immutable.kubeconfigPath, bctx)
}

// detectImmutableMaster decides how the very first node is created, so it runs
// before anything touches the infrastructure.
func (b *ClusterBootstrapper) detectImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if !immutable.IsImmutableMaster(ctx, bctx.metaConfig) {
		return nil
	}

	// Refused here rather than by a preflight: preflights can be skipped, and
	// the bootstrap this guards runs every phase to the end before dying on a
	// master address a non-cloud cluster never reports.
	if err := immutable.ValidateClusterType(ctx, bctx.metaConfig); err != nil {
		return err
	}

	dhlog.FromContext(ctx).InfoContext(ctx, "Master NodeGroup asks for an immutable system: bootstrapping without SSH and bashible")
	bctx.immutable = &immutableBootstrap{
		masterNodeName: firstMasterNodeName(bctx.metaConfig),
	}

	return nil
}

// applyImmutablePreflights adds the checks that only apply to an immutable
// master and drops the ones that reach the master over SSH, which it does not
// answer.
func (b *ClusterBootstrapper) applyImmutablePreflights(runner *preflight.Preflight, bctx *bootstrapContext) {
	if bctx.immutable == nil {
		return
	}

	runner.AddSuite(suites.NewImmutableSuite(suites.ImmutableDeps{
		MetaConfig:    bctx.metaConfig,
		BootstrapOpts: &b.Options.Bootstrap,
		GlobalOpts:    &b.Options.Global,
		CommanderMode: b.CommanderMode,
	}))

	// The cloud API check tunnels through the master host; there is no sshd
	// there to tunnel with.
	runner.DisableCheck(checks.CloudAPICheckName.String())
}

// buildImmutableMasterPayload renders the cloud-init the first master boots
// with, and "" on every other path. A progress line of its own: it decides what
// the machine will be, and it runs before the machine exists.
func (b *ClusterBootstrapper) buildImmutableMasterPayload(ctx context.Context, bctx *bootstrapContext, nodeName string) (string, error) {
	if bctx.immutable == nil {
		return "", nil
	}

	var cloudConfig string

	err := dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare immutable master payload", func(ctx context.Context) error {
		var err error
		cloudConfig, err = immutable.BuildMasterPayload(ctx, immutable.MasterPayloadInput{
			NodeName:      nodeName,
			MetaConfig:    bctx.metaConfig,
			StateCache:    bctx.stateCache,
			CandiDir:      b.Options.Global.CandiDir,
			GlobalOptions: &b.Options.Global,
		})
		return err
	})

	return cloudConfig, err
}

// connectToImmutableMaster collects the cluster credentials from the node and
// hands the rest of the bootstrap a Kubernetes client. No bashible pipeline:
// dhctl never sees a cluster key until the node gives it one.
func (b *ClusterBootstrapper) connectToImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.immutable.masterIP == "" {
		return errors.New("the first master address is unknown: rerun the bootstrap so the BaseInfra phase reports it")
	}

	// A rerun does not come back through the channel: once an attempt has
	// collected the credentials that channel is closed, and what it served is on
	// disk. Reading from there beats waiting on a listener that may be gone.
	complete, collectedPath, err := adminKubeconfigFromCache(ctx, bctx.stateCache)
	if err != nil {
		return err
	}
	if collectedPath != "" {
		bctx.immutable.kubeconfigPath = collectedPath
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"A previous attempt already collected the credentials; reusing the admin kubeconfig at %s", collectedPath,
		))
		if err := b.saveAdminKubeconfigOnRerun(ctx, complete, bctx, collectedPath); err != nil {
			return err
		}
		// Printed here too: the other two calls are the first-collection path and
		// the end of a successful run, so a stalled rerun would otherwise never
		// say where the credentials are.
		b.printHowToReachTheCluster(ctx, bctx.immutable.kubeconfigPath, bctx)
	}

	// The tunnel behind it stays open for the rest of the bootstrap.
	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.APIServerPort, "API server")
	if err != nil {
		return err
	}
	bctx.immutable.tunnelStop = stop
	server := "https://" + address

	if complete == nil {
		complete, err = b.collectImmutableCredentials(ctx, bctx)
		if err != nil {
			return err
		}
	}

	content, err := immutable.RetargetKubeconfig(ctx, complete, server, bctx.immutable.masterNodeName)
	if err != nil {
		return err
	}

	kubeconfigPath, err := b.writeImmutableKubeconfig(ctx, b.TmpDir, content)
	if err != nil {
		return err
	}

	kubeProvider, err := newKubeconfigKubeProvider(ctx, b, kubeconfigPath)
	if err != nil {
		return err
	}
	b.KubeProvider = kubeProvider

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return fmt.Errorf("connect to the API server of the immutable master at %s: %w", server, err)
	}

	// The credentials are stored and proven to work, so the node may stop
	// offering them. Attempted on a rerun too: a node that was already told
	// answers that it was.
	b.confirmImmutableHandoff(ctx, bctx)

	// Read exactly once, here — no later Client() call rebuilds from it. In
	// dhctl-server the process outlives the bootstrap, so leaving it to the
	// shutdown hook means admin credentials on disk for hours.
	removeImmutableKubeconfig(ctx, kubeconfigPath)

	return waitForImmutableMasterNode(ctx, kubeCl, bctx.immutable.masterNodeName)
}

// waitForImmutableMasterNode waits until kubelet has registered the node. The
// node also creates the bootstrap RBAC, the control-plane label and taint and
// the d8-pki Secret on its own; dhctl creates none of them.
func waitForImmutableMasterNode(ctx context.Context, kubeCl libcon.KubeClient, nodeName string) error {
	return libretry.NewLoop("Waiting for the first master node to register", waitNodeRegistered.attempts, waitNodeRegistered.interval).
		RunContext(ctx, func() error {
			_, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get node %s: %w", nodeName, err)
			}
			return nil
		})
}
