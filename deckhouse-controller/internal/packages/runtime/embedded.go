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
	"path/filepath"
	"strings"

	"github.com/ettle/strcase"
	"github.com/goccy/go-yaml"
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

	// bundleStaticFile is the static values file under embeddedDir whose
	// "<module>Enabled" keys declare which embedded modules to load.
	bundleStaticFile = "values.yaml"

	// enabledSuffix is the suffix on bundle keys that flag a module as enabled
	// (e.g. "deckhouseEnabled"); it is stripped to recover the module name.
	enabledSuffix = "Enabled"
)

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

	enabled, err := loadBundleEnabledMap(embeddedDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("load bundle enabled map: %w", err)
	}

	entries, err := os.ReadDir(embeddedDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("read dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		r.logger.Debug("load embedded module", slog.String("name", entry.Name()))

		conf, err := loader.LoadEmbeddedConf(ctx, embeddedDir+"/"+entry.Name(), r.logger)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("load embedded conf: %w", err)
		}

		if !enabled[strcase.ToCamel(conf.Definition.Name)] {
			continue
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

// loadBundleEnabledMap reads the bundle static file under dir and returns a map
// from camelCase module name to its enabled flag. It parses the YAML and keeps
// only keys ending in enabledSuffix, stripping the suffix to recover the
// camelCase module name (e.g. "nodeManagerEnabled" becomes "nodeManager"). The
// keys stay camelCase to match the bundle's own format; callers convert a
// module's name with strcase.ToCamel before looking it up.
func loadBundleEnabledMap(dir string) (map[string]bool, error) {
	bundleFile := filepath.Join(dir, bundleStaticFile)

	content, err := os.ReadFile(bundleFile)
	if err != nil {
		return nil, fmt.Errorf("read bundle file: %w", err)
	}

	result := make(map[string]bool)

	parsed := make(map[string]any)
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal bundle file: %w", err)
	}

	for k, v := range parsed {
		if before, ok := strings.CutSuffix(k, enabledSuffix); ok {
			result[before] = v.(bool)
		}
	}

	return result, nil
}
