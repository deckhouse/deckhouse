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
	"os"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// errImmutableMasterFailed is the node reporting that it has given up: nothing
// is still working towards a control plane, so the wait ends with the node's
// own message instead of half an hour of polling a dead node.
var errImmutableMasterFailed = errors.New("the first master gave up bringing the control plane up")

// adminKubeconfigFromCache returns the admin kubeconfig a previous attempt
// collected, or nil when there is nothing to reuse. A recorded path that no
// longer resolves is warned about: refusing would make the bootstrap dead.
func adminKubeconfigFromCache(ctx context.Context, stateCache state.Cache) ([]byte, string, error) {
	path, err := immutable.LoadCollectedKubeconfig(ctx, stateCache)
	if err != nil {
		return nil, "", err
	}
	if path == "" {
		return nil, "", nil
	}

	content, err := os.ReadFile(path)
	if err == nil {
		return content, path, nil
	}

	if handoverIsOver(ctx, stateCache) {
		return nil, "", fmt.Errorf(
			"read the admin kubeconfig %s a previous attempt collected: %w. "+
				"The first master has been told the credentials are stored, so it closed its bootstrap channel for good "+
				"and the installer's client key is gone with it: they cannot be collected a second time. "+
				"Restore that file, or destroy the cluster and bootstrap it again",
			path, err,
		)
	}

	dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
		"read the admin kubeconfig %s a previous attempt left behind: %v; collecting it from the node again", path, err,
	))
	return nil, "", nil
}

// handoverIsOver reports that an earlier attempt confirmed the handover: that is
// the one thing that blanks the installer's client key, and without the key a
// second collection could not be completed even if the node still answered.
func handoverIsOver(ctx context.Context, stateCache state.Cache) bool {
	material, err := immutable.LoadHandoffMaterial(ctx, stateCache)
	if err != nil || material == nil {
		return false
	}
	return material.ClientKeyPEM == ""
}

// collectImmutableCredentials waits for the node's bootstrap channel, collects
// the admin kubeconfig and stores the operator's copy. The node returns a
// certificate; it becomes credentials here, where the installer's key lives.
func (b *ClusterBootstrapper) collectImmutableCredentials(ctx context.Context, bctx *bootstrapContext) ([]byte, error) {
	kubeconfig, err := b.collectImmutableKubeconfig(ctx, bctx, immutable.HandoffPort)
	if err != nil {
		return nil, err
	}

	complete, err := immutable.CompleteAdminKubeconfig(ctx, bctx.stateCache, kubeconfig)
	if err != nil {
		return nil, err
	}

	// Stored first: these are the only credentials that will ever leave the node
	// and everything after this can fail. The operator's copy keeps the node's
	// own address; the retargeted one dies with this process's tunnel.
	if err := b.saveAdminKubeconfig(ctx, complete, bctx); err != nil {
		return nil, err
	}

	return complete, nil
}

// collectImmutableKubeconfig waits for the node's one-shot handoff endpoint and
// reads the admin kubeconfig out of it. This is the whole "wait for the node to
// install itself and bring a control plane up" step.
func (b *ClusterBootstrapper) collectImmutableKubeconfig(ctx context.Context, bctx *bootstrapContext, handoffPort int) ([]byte, error) {
	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil {
		return nil, err
	}
	if material == nil {
		return nil, errors.New("the bootstrap handoff credentials are missing from the state cache: rerun the bootstrap so the BaseInfra phase regenerates the master payload")
	}

	address, stop, err := b.openImmutableChannel(ctx, bctx, handoffPort, "credentials handoff")
	if err != nil {
		return nil, err
	}
	input := immutable.FetchKubeconfigInput{
		Address: address,
		// The endpoint's certificate is issued for the node's name, not for the
		// address dhctl dialled: that address did not exist when the payload
		// was built.
		ServerName: bctx.immutable.masterNodeName,
		Material:   material,
	}
	defer stop()

	// Narrated rather than silent: the node answers the status endpoint from the
	// moment it starts working, so an operator sees what it is doing and a node
	// that fails says why instead of just staying unreachable.
	var (
		kubeconfig  []byte
		lastMessage string
	)

	// Nothing here rebuilds the channel: the tunnel keeps its listener through a
	// broken connection and gossh's keepalive reconnects the client underneath it,
	// so a failed request is retried, not repaired.
	err = libretry.NewLoop("Waiting for the first master to bring the control plane up", waitAPIServerUp.attempts, waitAPIServerUp.interval).
		BreakIf(handoffGaveUp).
		RunContext(ctx, func() error {
			status, err := immutable.FetchStatus(ctx, input)
			if err != nil {
				return err
			}
			reportImmutableStatus(ctx, status, &lastMessage)
			if err := handoffReady(status); err != nil {
				return err
			}

			collected, err := immutable.FetchKubeconfig(ctx, input)
			if err != nil {
				return err
			}
			kubeconfig = collected
			return nil
		})
	if err != nil {
		return nil, err
	}

	return kubeconfig, nil
}

// handoffGaveUp reports the answers no amount of waiting changes: a payload the
// master never booted with, a channel that has already served, and a node that
// stopped working on the cluster.
func handoffGaveUp(err error) bool {
	if errors.Is(err, immutable.ErrHandoffUnauthorized) {
		return true
	}
	if errors.Is(err, immutable.ErrHandoffAlreadyServed) {
		return true
	}
	return errors.Is(err, errImmutableMasterFailed)
}

// reportImmutableStatus logs what the node says, once per distinct message.
func reportImmutableStatus(ctx context.Context, status *immutable.Status, lastMessage *string) {
	message := statusLine(status)
	if message == *lastMessage {
		return
	}
	*lastMessage = message
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("The first master reports: %s", message))
}

// handoffReady answers nil once the node will hand the credentials over. A node
// that says it failed says so for good: it stops working on the cluster, so
// polling out the rest of the budget only hides the message it already gave.
func handoffReady(status *immutable.Status) error {
	if status.Phase == immutable.PhaseFailed {
		return fmt.Errorf("%w: %s", errImmutableMasterFailed, statusLine(status))
	}
	if status.Phase == immutable.PhaseReady || status.Phase == immutable.PhaseCollected {
		return nil
	}
	return fmt.Errorf("the first master is not ready to hand the credentials over: %s", statusLine(status))
}

// statusLine renders what the node reports into one readable line.
func statusLine(status *immutable.Status) string {
	if status.Message == "" {
		return string(status.Phase)
	}
	return string(status.Phase) + ": " + status.Message
}

// confirmImmutableHandoff tells the node the credentials are stored, and only
// then does it stop offering them. Reported, not returned: the kubeconfig is
// already on disk, so a failed confirmation costs an open channel, not a cluster.
func (b *ClusterBootstrapper) confirmImmutableHandoff(ctx context.Context, bctx *bootstrapContext) {
	logger := dhlog.FromContext(ctx)

	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil {
		// Silence here costs a node that is never told, a cluster-admin key left
		// in the cache and a rerun that goes back to a dead channel.
		logger.WarnContext(ctx, fmt.Sprintf("load the handoff credentials to confirm the handover: %v", err))
		return
	}
	if material == nil {
		logger.WarnContext(ctx, "The handoff credentials are gone from the state cache; the first master will keep its bootstrap channel open until it times out.")
		return
	}

	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.HandoffPort, "handoff confirmation")
	if err != nil {
		logger.WarnContext(ctx, fmt.Sprintf("confirm the handover to the first master: %v", err))
		return
	}
	defer stop()

	input := immutable.FetchKubeconfigInput{Address: address, ServerName: bctx.immutable.masterNodeName, Material: material}
	switch err := immutable.ConfirmCollected(ctx, input); {
	case err == nil:
		logger.InfoContext(ctx, "Confirmed the handover; the first master closed its bootstrap channel.")
	case errors.Is(err, immutable.ErrHandoffAlreadyServed):
		// A rerun reaching a channel an earlier attempt already closed. That is
		// the confirmation, arriving as the node's memory of it.
		logger.InfoContext(ctx, "The first master had already closed its bootstrap channel.")
	default:
		logger.WarnContext(ctx, fmt.Sprintf("confirm the handover to the first master, it will keep the bootstrap channel open: %v", err))
		return
	}

	// The rest of the material stays: a rerun that found it gone would mint fresh
	// material and render a payload the master never booted with. Only the
	// client key goes — see immutable.ForgetHandoffClientKey.
	if err := immutable.ForgetHandoffClientKey(ctx, bctx.stateCache); err != nil {
		logger.WarnContext(ctx, fmt.Sprintf("drop the installer's client key from the state cache: %v", err))
	}
}
