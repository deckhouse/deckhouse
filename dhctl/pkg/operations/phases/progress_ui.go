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

package phases

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

// RunProgress opens a terminal progress session named name, runs body (which
// emits Progress events into the supplied channel), and closes the session
// afterwards. A background goroutine consumes the channel and advances the bar.
//
// The "current action" spinner that the legacy bar fed from a label channel is
// now driven automatically by dhlog.RunProcess markers, so no label channel is
// needed here.
func RunProgress(ctx context.Context, l *slog.Logger, name string, body func(progressCh chan Progress) error) error {
	progressCh, finish := InitProgress(ctx, l, name)
	defer finish()

	return body(progressCh)
}

// InitProgress opens a terminal progress session named name and starts a
// background goroutine that consumes Progress events sent on the returned
// channel, advancing the bar. The returned finish function closes the channel,
// waits for the consumer to drain, and closes the progress session. It is safe
// to call finish exactly once (e.g. via defer).
//
// This mirrors the contract of the legacy deferred-finish progress bar
// initializer, but drives the slog TerminalUIHandler instead of the pterm bar.
func InitProgress(ctx context.Context, l *slog.Logger, name string) (chan Progress, func()) {
	progressCh := make(chan Progress, 5)

	dhlog.StartProgress(ctx, l, name)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumeProgress(ctx, l, progressCh, stop)
	}()

	var finished bool
	finish := func() {
		if finished {
			return
		}
		finished = true

		// Signal the consumer to stop, but do NOT close progressCh: the pipeline's deferred
		// Finalize can still emit a final Progress event after the body (and runProgress) has
		// returned. Senders use a non-blocking send, so a late event to the un-drained, never-
		// closed channel neither panics nor blocks.
		close(stop)
		<-done

		dhlog.FinishProgress(ctx, l)
	}

	return progressCh, finish
}

// consumeProgress reads Progress events and advances the new progress bar. The
// increment math is ported verbatim from the legacy updateProgress loop: the
// legacy bar operated on an integer scale of 0..100, here we keep the same
// integer math and report the resulting fraction (current/100) to
// dhlog.Progress.
func consumeProgress(ctx context.Context, l *slog.Logger, progressCh chan Progress, stop <-chan struct{}) {
	inc := 0
	lastCompleted := ""
	current := 0 // 0..100, mirrors the legacy pterm bar's Current.

	for {
		var msg Progress
		select {
		case <-stop:
			return
		case msg = <-progressCh:
		}

		if inc == 0 || lastCompleted == "" {
			// calculate increment
			phasesCount := len(msg.Phases)
			for _, p := range msg.Phases {
				phasesCount += len(p.SubPhases)
			}
			if phasesCount > 0 {
				inc = 100 / phasesCount
			}

			text := phaseToString(msg, false)
			if text != "" {
				dhlog.Progress(ctx, l, float64(current)/100, text)
			}
		}

		if msg.CompletedPhase != "" {
			completed := phaseToString(msg, true)

			if completed == lastCompleted {
				continue
			}

			increment := int(math.Round(msg.Progress*100) - float64(current))

			if increment == 0 {
				increment = inc
			}

			if current+increment > 100 {
				increment = 100 - current
			}

			current += increment
			lastCompleted = completed

			// The successful phase transition is THE only thing tagged for the compact view.
			l.InfoContext(ctx, strings.TrimSpace(completed), dhlog.ShowInCompacted(), dhlog.BadgeSuccess())
			dhlog.Progress(ctx, l, float64(current)/100, phaseToString(msg, false))
		}
	}
}

// phaseToString maps a Progress event to a human-readable bar title. Ported
// verbatim from the legacy phaseToString in the pkg/util progress bar.
func phaseToString(p Progress, completed bool) string {
	// Butify bootstrap: phases with subphases
	phasesMap := make(map[OperationPhase]string)
	phasesMap[PreInfraPreflightsPhase] = "Common preflight checks"
	phasesMap[PostInfraPreflightsPhase] = "Static and post-infra preflight checks"
	phasesMap[BaseInfraPhase] = "Base Infrastructure"
	phasesMap[InstallKubernetesPhase] = "Install Kubernetes on the first master node"
	phasesMap[InstallDeckhousePhase] = "Install Deckhouse"
	phasesMap[CreateResourcesPhase] = "Create resources"
	phasesMap[InstallAdditionalMastersAndStaticNodes] = "Install additional master nodes and CloudPermanent nodes"
	phasesMap[ExecPostBootstrapPhase] = "Execute post-bootstrap script"
	phasesMap[DeleteResourcesPhase] = "Delete resources"
	phasesMap[AllNodesPhase] = "Process all nodes"
	phasesMap[FinalizationPhase] = "Finalization"

	phasesMap[ConvergeCheckPhase] = "Check converge"
	phasesMap[ScaleToMultiMasterPhase] = "Scale cluster to multimaster"
	phasesMap[ScaleToSingleMasterPhase] = "Scale cluster to singlemaster"
	phasesMap[DeckhouseConfigurationPhase] = "Configure Deckhouse"

	phasesMap[CreateStaticDestroyerNodeUserPhase] = "Create NodeUser for static destroyer"
	phasesMap[UpdateStaticDestroyerIPs] = "Update static destroyer IPs"
	phasesMap[WaitStaticDestroyerNodeUserPhase] = "Wait for NodeUser"
	phasesMap[SetDeckhouseResourcesDeletedPhase] = "Set Deckhouse resources to deleted"
	phasesMap[CommanderUUIDWasChecked] = "Commander UUID was checked"

	subphasesMap := make(map[OperationSubPhase]string)
	subphasesMap[InstallDeckhouseSubPhaseConnect] = "Connect to master host"
	subphasesMap[InstallDeckhouseSubPhaseInstall] = "Install..."
	subphasesMap[InstallDeckhouseSubPhaseWait] = "Wait for the first master readiness"
	subphasesMap[OperationSubPhase(CheckInfra)] = "Check Infrastructure"
	subphasesMap[OperationSubPhase(CheckConfiguration)] = "Check configuration"
	subphasesMap[BaseInfraSubPhaseBaseInfra] = "Base Infrastructure"
	subphasesMap[BaseInfraSubPhaseFirstMaster] = "First master node"
	subphasesMap[InstallKubernetesSubPhaseBundlePreparation] = "Prepare bashible bundle"
	subphasesMap[InstallKubernetesSubPhaseRegistryPackagesProxy] = "Prepare registry packages proxy"
	subphasesMap[InstallKubernetesSubPhaseNodePreparation] = "Prepare node"
	subphasesMap[InstallKubernetesSubPhaseExecuteBashibleBundle] = "Execute bashible bundle"
	subphasesMap[InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters] = "Install additional master nodes"
	subphasesMap[InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes] = "Install additional static nodes"
	subphasesMap[InstallAdditionalMastersAndStaticNodesSubPhaseWait] = "Wait for control plane manager become ready"

	msg := ""
	if completed {
		if p.CompletedSubPhase != "" {
			msg = fmt.Sprintf("%s: %s", phasesMap[p.CurrentPhase], subphasesMap[p.CompletedSubPhase])
		} else {
			msg = phasesMap[p.CompletedPhase]
		}
	} else {
		if p.CurrentSubPhase != "" {
			msg = fmt.Sprintf("%s: %s", phasesMap[p.CurrentPhase], subphasesMap[p.CurrentSubPhase])
		} else {
			if p.CurrentPhase != "" {
				msg = phasesMap[p.CurrentPhase]
			} else {
				msg = phasesMap[p.CompletedPhase]
			}
		}
	}

	return fmt.Sprintf("%-60s", msg)
}
