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

// The boot: restoring the checkpoints before anything else runs, and the boot gate (the scheduler's
// single Resume).

package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// globalSyncTimeout bounds the boot gate's wait for Global's synchronization. A global hook whose
// synchronization keeps failing is retried by its lane forever, so without a bound deckhouse would
// stay up, never booted, reporting nothing. The old runtime went on without the modules after its
// 10 minutes; Boot fails instead, because Global is the barrier every package runs behind and going
// on without its values is not an option: the restart (or another replica) is the visible way out.
const globalSyncTimeout = 10 * time.Minute

// tracerName is the instrumentation scope of the Boot spans.
const tracerName = "package-engine"

// ErrGlobalNotSynced is Boot's error when Global's hooks did not synchronize within
// globalSyncTimeout.
var ErrGlobalNotSynced = errors.New("global hooks not synchronized")

// BootGlobal is Global's boot input, a type of its own so there is no room for a missing,
// duplicated or attached Global.
type BootGlobal struct {
	Module global.Module
	// Settings is Global's settings checkpoint; nil makes Global wait for the first Apply.
	Settings *model.Settings
}

// resumeState orders the scheduler's Resume against Boot and Engine.Resume; Scheduler.Resume is
// idempotent. It has its own mutex because Scheduler.Resume is called under it, and e.mu is never
// held across a scheduler call.
type resumeState struct {
	mu sync.Mutex
	// booted: Boot has started the restored packages (without a Global); Resume may resume.
	booted    bool
	requested bool // requested: Engine.Resume came before booted; Boot resumes then.
}

// Boot restores the checkpoints persisted before a restart and starts them, before any controller
// starts. Blocking; called once, with the scheduler paused. bootGlobal nil boots no Global (no
// global node, no barrier): the engine without EnableModuleV2, which runs applications only.
//  0. Refuse a package named global or a name passed twice: AddNode replaces by name, so either
//     would take another's node, and the registry would lose a package Reconciler Shutdown must
//     stop.
//  1. Add the global node and start Global, so its sync (the hooks) runs while packages restore.
//  2. Restore every package with a Version (lifecycle.Restore), in parallel; the first error is
//     returned and deckhouse must exit. A package being deleted is restored too, to be uninstalled.
//  3. Start a package Reconciler for every restored package; its Current's facts accumulate while
//     paused. A removing one starts removing; readiness does not wait for it.
//  4. Set the modules' intents.
//  5. The boot gate: with a Global, wait until its hooks are synchronized, bounded by ctx and by
//     globalSyncTimeout; without one, return with the scheduler paused until Engine.Resume.
//  6. With a Global, Resume.
//
// Why 4 and 5 come before Resume: an unknown intent falls to the module floor (Disable), and an
// unknown gate input (the Kubernetes version global discovery fills) reads as Forbid; the first
// computation would disable modules and delete their releases on every restart.
//
// Restored Currents run once after boot with their checkpoint, without waiting for Apply (applied
// is not persisted); an unchanged release makes that run cheap.
func (e *Engine) Boot(ctx context.Context, bootGlobal *BootGlobal, packages []model.BootPackage) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Boot", trace.WithAttributes(
		attribute.Int("packages", len(packages)),
		attribute.Bool("global", bootGlobal != nil)))
	defer span.End()

	err := e.boot(ctx, bootGlobal, packages)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// boot is Boot under its span.
func (e *Engine) boot(ctx context.Context, bootGlobal *BootGlobal, packages []model.BootPackage) error {
	seen := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		if pkg.Name == schedule.GlobalNode {
			return fmt.Errorf("boot package %q: the name is reserved for Global", pkg.Name)
		}

		if _, ok := seen[pkg.Name]; ok {
			return fmt.Errorf("boot package %q: the name is passed twice", pkg.Name)
		}

		seen[pkg.Name] = struct{}{}
	}

	var g *global.Global

	if bootGlobal != nil {
		// Global's facts and its node carry one version value.
		version := bootGlobal.Module.GetVersion().Original()

		if err := e.scheduler.AddNode(global.Node(bootGlobal.Module.GetVersion(), version)); err != nil {
			return fmt.Errorf("add global node: %w", err)
		}

		g = global.New(global.Config{
			Module:   bootGlobal.Module,
			Version:  version,
			Settings: checkpointAt(bootGlobal.Settings, bootGlobal.Module.LatestSettingsVersion()),
			Notify:   e.notifier(schedule.GlobalNode),
		}, e.deps.Global)

		e.mu.Lock()
		e.global = g
		g.Start(e.ctx)
		e.mu.Unlock()
	}

	restoreCtx, restoreSpan := otel.Tracer(tracerName).Start(ctx, "restore")
	restored, err := e.restore(restoreCtx, packages)
	restoreSpan.End()

	if err != nil {
		return err
	}

	e.mu.Lock()
	e.expected = make(map[string]struct{}, len(packages))
	for i, pkg := range packages {
		if restored[i] == nil {
			continue
		}

		e.startLocked(pkg.Name, pkg.Kind, restored[i])

		if pkg.Kind == model.KindModule && !pkg.Removing {
			e.expected[pkg.Name] = struct{}{}
		}
	}
	e.mu.Unlock()

	for _, pkg := range packages {
		if pkg.Intent != nil {
			e.scheduler.SetIntent(pkg.Name, pkg.Intent)
		}
	}

	if g == nil {
		e.resume.mu.Lock()
		e.resume.booted = true
		if e.resume.requested {
			e.scheduler.Resume()
		}
		e.resume.mu.Unlock()

		return nil
	}

	// The deadline counts from here, not from the start of Boot: a restore that downloads images on
	// a cold node may take long on its own, and its failures end Boot anyway.
	deadline := retry.OrWall(e.deps.Global.Clock).NewTimer(globalSyncTimeout)
	defer deadline.Stop()

	_, waitSpan := otel.Tracer(tracerName).Start(ctx, "waitGlobalSync")
	defer waitSpan.End()

	select {
	case <-g.Synced():
	case <-deadline.C():
		return fmt.Errorf("%w within %s", ErrGlobalNotSynced, globalSyncTimeout)
	case <-ctx.Done():
		return fmt.Errorf("%w before boot ended: %w", ErrGlobalNotSynced, context.Cause(ctx))
	}

	e.scheduler.Resume()

	return nil
}

// Resume ends the boot gate of an Engine without a Global; the wiring calls it when addon-operator
// has converged. Why wait: the gate inputs then come from addon-operator's global values, known
// only after its first converge, and an unknown input reads as Forbid, so an earlier computation
// would disable every restored application and delete its release. A call during Boot is deferred
// to its end, so the first computation sees every restored node. Later converges do not pause the
// engine (the inputs stay known); Recheck follows them. Idempotent; a no-op with a Global.
func (e *Engine) Resume() {
	e.resume.mu.Lock()
	defer e.resume.mu.Unlock()

	if !e.resume.booted {
		e.resume.requested = true
		return
	}

	e.scheduler.Resume()
}

// restore's result is indexed like packages (nil: no Version); each goroutine writes only its own
// index.
func (e *Engine) restore(ctx context.Context, packages []model.BootPackage) ([]*model.Restored, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(defaultRestoreParallelism)

	restored := make([]*model.Restored, len(packages))

	for i, pkg := range packages {
		if pkg.Version == "" {
			continue
		}

		// After a failure the remaining restores start on the cancelled ctx and return at once.
		g.Go(func() error {
			rev, err := lifecycle.Restore(ctx, e.deps.Reconciler, pkg)
			if err != nil {
				return fmt.Errorf("restore package %q: %w", pkg.Name, err)
			}

			restored[i] = &model.Restored{
				Revision:      *rev,
				Configuration: checkpointConfiguration(pkg, latestSettingsVersion(rev.Package)),
				Removing:      pkg.Removing,
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return restored, nil
}

// checkpointConfiguration is the configuration a restored Current starts with, its settings at the
// restored object's latest settings version; nil when the settings checkpoint is absent.
func checkpointConfiguration(pkg model.BootPackage, latest int) *model.Configuration {
	settings := checkpointAt(pkg.Settings, latest)
	if settings == nil {
		return nil
	}

	return &model.Configuration{
		Settings:         *settings,
		Maintenance:      pkg.Maintenance,
		ResourceRequests: pkg.ResourceRequests,
	}
}

// checkpointAt is a copy of a settings checkpoint at version latest: the checkpoint holds effective
// values, already in the restored object's latest schema, so they are never converted again.
// nil stays nil.
func checkpointAt(settings *model.Settings, latest int) *model.Settings {
	if settings == nil {
		return nil
	}

	at := *settings
	at.SchemaVersion = latest

	return &at
}

// versionedSettings is implemented by objects with a versioned settings schema (modules, applications).
type versionedSettings interface {
	LatestSettingsVersion() int
}

// latestSettingsVersion is obj's latest settings-schema version; 0 for an object without versioned
// settings.
func latestSettingsVersion(obj model.Package) int {
	if v, ok := obj.(versionedSettings); ok {
		return v.LatestSettingsVersion()
	}

	return 0
}
