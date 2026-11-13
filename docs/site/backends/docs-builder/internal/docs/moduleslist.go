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

package docs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/fsync"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// modulesListData is the data of the list of modules for the templates
// (hugo.Data.modules_list).
const modulesListData = "data/modules_list.json"

// RenderModulesList renders the page of the list of modules from data (the
// cards, see hugo.Data.modules_list) as a Hugo site of its own in siteDir,
// built from the site template in initDir, and returns the page of every
// language. siteDir is left for the caller to remove.
func RenderModulesList(ctx context.Context, initDir, siteDir string, data []byte, logger *log.Logger) (map[string][]byte, error) {
	syncer := fsync.NewSyncer()
	syncer.NoChmod = true
	syncer.NoTimes = true
	if err := syncer.Sync(siteDir, initDir); err != nil {
		return nil, fmt.Errorf("copy site template: %w", err)
	}

	// Only the page of the list: the search index and the AI manifest
	// belong to the versions.
	for _, dir := range []string{"content/search", aiContentDir} {
		if err := os.RemoveAll(filepath.Join(siteDir, dir)); err != nil {
			return nil, fmt.Errorf("remove %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(siteDir, modulesListData), data, 0600); err != nil {
		return nil, fmt.Errorf("write modules list data: %w", err)
	}

	site := &Service{
		baseDir: siteDir,
		logger:  logger,
	}
	if err := site.buildHugo(ctx); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("hugo build: %w", err)
		}
		return nil, fmt.Errorf("%w: %w", ErrBuild, err)
	}

	pages := make(map[string][]byte, len(Languages))
	for _, lang := range Languages {
		page, err := os.ReadFile(filepath.Join(siteDir, "public", lang, "modules", "index.html"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s page: %w", lang, err)
		}
		pages[lang] = page
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("%w: no page of the list of modules", ErrBuild)
	}

	return pages, nil
}
