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

package modulesync

// This file resolves origins from the running Deckhouse image - the one
// permanent origin source: embedded modules follow the image on every
// upgrade.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
)

const (
	// embeddedLoadWorkers caps how many embedded modules are read from disk concurrently.
	embeddedLoadWorkers = 8

	// embeddedRepositoryName stands for the Deckhouse image itself and resolves to no
	// PackageRepository - unlike `deckhouse`, which is a name a real repository may take.
	embeddedRepositoryName = "embedded"
)

// nonModuleDirs are directories in the embedded modules dir that do not
// hold a module: shared helm templates and registry packages.
var nonModuleDirs = []string{
	"000-common",
	"007-registrypackages",
}

// originsFromImage returns an origin for every module the running image ships.
func (s *Syncer) originsFromImage(ctx context.Context) (map[string]Origin, error) {
	s.logger.Debug("load embedded modules", slog.String("path", s.embeddedDir))

	entries, err := os.ReadDir(s.embeddedDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	// only a definition names its module; each goroutine owns one slot, so the names need no lock
	names := make([]string, len(entries))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(embeddedLoadWorkers)

	for i, entry := range entries {
		if !entry.IsDir() || slices.Contains(nonModuleDirs, entry.Name()) {
			continue
		}

		g.Go(func() error {
			// bail out before any work if another module already failed or the caller cancelled
			if err := ctx.Err(); err != nil {
				return err
			}

			s.logger.Debug("load embedded module", slog.String("name", entry.Name()))

			conf, err := loader.LoadEmbeddedConf(ctx, s.embeddedDir+"/"+entry.Name(), s.logger)
			if err != nil {
				return fmt.Errorf("load embedded conf: %w", err)
			}

			names[i] = conf.Definition.Name

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	origins := make(map[string]Origin, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}

		// an embedded module carries the running Deckhouse version - the runtime's edition version verbatim
		origins[name] = Origin{RepositoryName: embeddedRepositoryName, PackageVersion: app.Version, Embedded: true}
	}

	return origins, nil
}
