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

package pkgsync

// This file reads the embedded modules dir once per sync: the version pass
// and the module pass consume the same walk.

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// embeddedModule is one module the running image ships: its dir name, which
// carries the weight prefix, and its loaded definition.
type embeddedModule struct {
	dirName string
	def     *dto.ModuleDefinition
}

// loadEmbeddedModules walks the embedded modules dir and loads a definition
// per module dir. A dir with no readable definition names no module and is
// skipped with a warning: the module then gets neither a package version nor
// a Module resource, and the runtime could not run it anyway.
func (s *syncer) loadEmbeddedModules() ([]embeddedModule, error) {
	entries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded modules dir: %w", err)
	}

	modules := make([]embeddedModule, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(app.DummyModules, entry.Name()) {
			continue
		}

		moduleDir := filepath.Join(s.embeddedModulesDir, entry.Name())

		def, err := loader.LoadEmbeddedDefinition(moduleDir)
		if err != nil {
			s.logger.Warn("module dir holds no readable definition, skip the module",
				slog.String("dir", moduleDir), log.Err(err))

			continue
		}

		modules = append(modules, embeddedModule{dirName: entry.Name(), def: def})
	}

	return modules, nil
}
