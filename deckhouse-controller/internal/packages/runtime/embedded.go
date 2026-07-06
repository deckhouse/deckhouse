// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

const (
	// embeddedDir is the directory, relative to the working directory, that
	// holds embedded modules shipped with the controller.
	embeddedDir = "modules"

	// embeddedLoadWorkers caps how many embedded modules are loaded
	// concurrently in loadEmbedded.
	embeddedLoadWorkers = 8
)

// dummyModules are modules that should be skipped.
var dummyModules = []string{
	"000-common",
	"007-registrypackages",
}

// loadGlobal loads the global module from the embedded directory and registers
// it in the status service and the package store. Scheduler wiring happens
// later in buildScheduler/AddNode, not here.
func (r *Runtime) loadGlobal(ctx context.Context) error {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadGlobal")
	defer span.End()

	r.logger.Debug("load global package")

	conf, err := loader.LoadGlobalConf(ctx, r.logger)
	if err != nil {
		return fmt.Errorf("load global conf: %w", err)
	}

	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager

	r.global, err = global.NewModuleByConfig(conf, r.logger)
	if err != nil {
		return fmt.Errorf("new global module: %w", err)
	}

	r.status.NewStatus(r.global.GetName())
	r.status.SetConditionTrue(r.global.GetName(), status.ConditionRequirementsMet)
	r.status.SetConditionTrue(r.global.GetName(), status.ConditionReadyOnFilesystem)
	r.status.SetConditionTrue(r.global.GetName(), status.ConditionLoaded)
	r.packages.Update(r.global.GetName(), r.global.GetVersion().String(), make(addonutils.Values), "")

	return nil
}

// loadEmbedded discovers embedded modules under embeddedDir and registers the
// ones enabled by the bundle. It reads the bundle's enabled map, then for each
// module directory builds the module from its on-disk config, wires the
// runtime's shared managers into it, and stores it in the runtime's module map.
// Modules not marked enabled in the bundle are skipped.
func (r *Runtime) loadEmbedded(ctx context.Context) error {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadEmbedded")
	defer span.End()

	span.SetAttributes(attribute.String("path", embeddedDir))

	r.logger.Debug("load embedded modules", slog.String("path", embeddedDir))

	entries, err := os.ReadDir(embeddedDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("read dir: %w", err)
	}

	// Each module is independent: load its config, wire the runtime's shared
	// managers, build it and store it. Run them concurrently and let the first
	// failure cancel the rest.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(embeddedLoadWorkers)

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(dummyModules, entry.Name()) {
			continue
		}

		g.Go(func() error {
			// Bail out early if another module already failed (errgroup cancels
			// ctx) or the caller cancelled, before doing any work.
			if err := ctx.Err(); err != nil {
				return err
			}

			r.logger.Debug("load embedded module", slog.String("name", entry.Name()))

			conf, err := loader.LoadEmbeddedConf(ctx, embeddedDir+"/"+entry.Name(), r.logger)
			if err != nil {
				return fmt.Errorf("load embedded conf: %w", err)
			}

			conf.Patcher = r.objectPatcher
			conf.ScheduleManager = r.scheduleManager
			conf.KubeEventsManager = r.kubeEventsManager
			conf.GlobalValuesGetter = r.global.GetValues
			// TODO(ipaqsa): set deckhouse version instead
			conf.Definition.Version = "v0.0.0"

			module, err := modules.NewModuleByConfig(conf.Definition.Name, conf, r.logger)
			if err != nil {
				return fmt.Errorf("new module by config: %w", err)
			}

			r.mu.Lock()
			r.modules[module.GetName()] = module
			r.mu.Unlock()

			// register package in status and packages stores
			r.status.NewStatus(module.GetName())
			r.status.SetConditionTrue(module.GetName(), status.ConditionRequirementsMet)
			r.status.SetConditionTrue(module.GetName(), status.ConditionReadyOnFilesystem)
			r.status.SetConditionTrue(module.GetName(), status.ConditionLoaded)
			r.status.UpdateVersion(module.GetName(), module.GetVersion().String())
			r.packages.Update(module.GetName(), module.GetVersion().String(), make(addonutils.Values), "")

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}
