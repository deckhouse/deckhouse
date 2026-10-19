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

// The activation of a prepared revision and its scheduler node, and the restore of the Current
// revision at boot: the only two ways a node's revision changes, each by AddNode.

package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/current"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/gate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/script"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// detachTimeout bounds a detach, which ignores the caller's cancellation so that shutdown and
// remove detach too.
const detachTimeout = 30 * time.Second

// activate makes the prepared revision Current, with the configuration that passed every gate:
//  1. AddNode it; a rejection changes nothing (rejectCandidate);
//  2. take what the candidate prepared and stop the old Current;
//  3. report CurrentVersion and clear the Candidate section in one write: the point of no return,
//     and the status never shows the activated version as a pending candidate;
//  4. start the new Current and push it config;
//  5. detach the old Current's image.
//
// AddNode comes first because it is the last check that can fail (another package's activation
// may have closed a cycle since the final gate): a rejection leaves the old Current running and
// nothing to roll back. Until the old Current stops the scheduler ignores its facts: the node
// already carries the new revision, and facts of another revision are not the node's. Global may
// already ensure the candidate's CRDs, which the new Current needs anyway.
//
// ctx is checked before AddNode (a shutdown must not change the graph; the loop's shutdown
// releases the candidate) and after the stop (a shutdown must not report; the next boot restores
// the version the status still holds).
//
// The reverse-dependency check (does every enabled dependent accept the new version?) is the final
// gate's (gate.Decide), not repeated here: Restore adds nodes through AddNode too.
func (r *Reconciler) activate(ctx context.Context, config *model.Configuration, checksum string) {
	c := r.candidate

	if ctx.Err() != nil {
		return
	}

	rev := c.prepared.revision()

	if err := r.deps.Scheduler.AddNode(schedulerNode(r.kind, r.name, rev)); err != nil {
		r.rejectCandidate(ctx, rev.Image.Version, err)
		return
	}

	r.takeCandidate()

	previous := r.current
	previous.Stop()
	r.current = nil

	if ctx.Err() != nil {
		// Shutting down after the graph changed: report nothing; the cleanup before boot handles
		// both images.
		return
	}

	r.reporter.SetActivated(rev.Image.Version)
	r.logger.Info("activated", slog.String("version", rev.Image.Version))

	r.current = r.startCurrent(ctx, rev, false)
	r.activated.Store(&rev)
	r.current.Push(config, checksum, nil)

	if previous != nil {
		r.detach(ctx, previous.Revision().Image)
	}
}

// rejectCandidate keeps the prepared candidate and the old Current; the Candidate section reports
// RequirementsNotMet, as the final gate does for a blocked prepared revision, and the final gate
// decides again on the next wake.
func (r *Reconciler) rejectCandidate(ctx context.Context, version string, err error) {
	if ctx.Err() != nil {
		return
	}

	r.logger.Warn("activation rejected by the scheduler, current is kept", slog.String("version", version), log.Err(err))

	r.reporter.SetCandidate(model.CandidateReport{
		Version: version,
		Blocked: model.ReasonRequirementsNotMet,
		Reason:  gate.ConstraintReason(err),
		Message: gate.ConstraintMessage(err),
	})
}

// detach keeps the stored image (see detachTimeout); a failure is only logged.
func (r *Reconciler) detach(ctx context.Context, img model.Image) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), detachTimeout)
	defer cancel()

	if err := detachImage(ctx, r.deps.Disks, img); err != nil {
		r.logger.Warn("detach image", slog.String("version", img.Version), log.Err(err))
	}
}

// restored: rev is started from a settings checkpoint (current.Config.Restored).
func (r *Reconciler) startCurrent(ctx context.Context, rev model.Revision, restored bool) *current.Reconciler {
	return current.Start(ctx, current.Config{
		Name:     r.name,
		Kind:     r.kind,
		Revision: rev,
		Restored: restored,
	}, r.deps.Current, r.reporter)
}

type scriptPackage interface {
	GetEnabledScriptDescriptor() *script.Descriptor
}

// schedulerNode builds the node of a loaded revision (with global.Node, the only place nodes are
// built).
//   - Every package subscribes to global: it renders against the global values (without a Global
//     the subscription adds nothing).
//   - Only modules are marked Module and carry their ModuleView: Global ensures CRDs and webhooks
//     of modules only, from the view its event brings.
//   - The revision is the image's version, so a new version reruns Global.
//   - The enabled script reads the object's live settings, which Current applies on accept.
func schedulerNode(kind model.Kind, name string, rev model.Revision) schedule.Node {
	var obj packageObject = rev.Package

	loaded := obj.GetConstraints()

	// The object's constraints may be shared: copy the map, never write it.
	c := loaded
	c.Subscriptions = make(map[string]struct{}, len(loaded.Subscriptions)+1)
	maps.Copy(c.Subscriptions, loaded.Subscriptions)
	c.Subscriptions[schedule.GlobalNode] = struct{}{}

	n := schedule.Node{
		Name:        name,
		Version:     obj.GetVersion(),
		Revision:    rev.Image.Version,
		Module:      kind == model.KindModule,
		Constraints: c,
		Output:      current.OutputOf(rev.Package),
	}

	if !n.Module {
		return n
	}

	if s, ok := obj.(scriptPackage); ok {
		n.EnabledScript = s.GetEnabledScriptDescriptor
	}

	// Not Image.Path: an embedded module's image is <EmbeddedDir>/<name>, while its directory has a
	// weight prefix (modules/002-deckhouse) only the loader resolves; Global would find no crds
	// directory there and silently install no CRD.
	n.Attachment = model.ModuleView{
		Name:    name,
		Version: rev.Image.Version,
		Path:    obj.GetPath(),
		Package: rev.Package,
	}

	return n
}

// Restore brings back the Current revision of one package at boot: the preparation steps once, in
// place, then AddNode, while the scheduler is paused. Attach verifies against the registry, so a
// registry outage at restart makes the package unrestorable and Deckhouse exits until it is back
// (accepted). On error the caller must exit; the cleanup before the next boot removes what was
// attached.
func Restore(ctx context.Context, deps Deps, pkg model.BootPackage) (*model.Revision, error) {
	if pkg.Kind != model.KindApplication && pkg.Kind != model.KindModule {
		return nil, fmt.Errorf("restore %s@%s: unknown package kind %q", pkg.Name, pkg.Version, pkg.Kind)
	}

	var st prepared

	t := target{Kind: pkg.Kind, Name: pkg.Name, Version: pkg.Version, Source: pkg.Source, Embedded: pkg.Embedded}
	for _, step := range prepareSteps(deps, t, &st) {
		if err := step.Run(ctx); err != nil {
			return nil, fmt.Errorf("restore %s@%s: %w", pkg.Name, pkg.Version, err)
		}
	}

	rev := st.revision()

	if err := deps.Scheduler.AddNode(schedulerNode(pkg.Kind, pkg.Name, rev)); err != nil {
		return nil, fmt.Errorf("restore %s@%s: add node: %w", pkg.Name, pkg.Version, err)
	}

	return &rev, nil
}
