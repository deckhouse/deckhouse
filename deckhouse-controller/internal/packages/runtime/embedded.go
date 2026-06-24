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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
)

const (
	// embeddedDir is the directory, relative to the working directory, that
	// holds embedded modules shipped with the controller.
	embeddedDir = "modules"
)

var dummyModules = []string{
	"000-common",
	"007-registrypackages",
}

// loadEmbedded discovers embedded modules under embeddedDir, builds each one
// from its on-disk config, wires the runtime's shared managers into it, and
// registers the resulting modules in the runtime's module map.
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

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(dummyModules, entry.Name()) {
			continue
		}

		r.logger.Debug("load embedded module", slog.String("name", entry.Name()))

		conf, err := loader.LoadEmbeddedConf(ctx, embeddedDir+"/"+entry.Name(), r.logger)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("load embedded conf: %w", err)
		}

		conf.Patcher = r.objectPatcher
		conf.ScheduleManager = r.scheduleManager
		conf.KubeEventsManager = r.kubeEventsManager
		conf.GlobalValuesGetter = r.global.GetValues

		module, err := modules.NewModuleByConfig(conf.Definition.Name, conf, r.logger)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("new module by config: %w", err)
		}

		r.mu.Lock()
		r.modules[module.GetName()] = module
		r.mu.Unlock()
	}

	return nil
}
