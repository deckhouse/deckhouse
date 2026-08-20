// Copyright 2023 Flant JSC
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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/fsync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/pkg/hugo"
)

func (svc *Service) Build(ctx context.Context) error {
	// Acquire the build slot. Selecting on ctx means a caller whose request
	// was already canceled — e.g. the client hit its timeout while an earlier
	// build still held the slot — drops out here instead of queuing up yet
	// another redundant full-site rebuild behind it.
	select {
	case svc.buildSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-svc.buildSem }()

	start := time.Now()
	status := "ok"
	defer func() {
		dur := time.Since(start).Seconds()
		svc.metrics.CounterAdd(metrics.DocsBuilderBuildTotal, 1, map[string]string{"status": status})
		svc.metrics.HistogramObserve(metrics.DocsBuilderBuildDurationSeconds, dur, map[string]string{"status": status}, nil)
	}()

	// The request may have been canceled while we waited for the slot; skip
	// the build rather than start work nobody is waiting for.
	if err := ctx.Err(); err != nil {
		status = "canceled"
		return err
	}

	err := svc.buildHugo(ctx)
	if err != nil {
		// A canceled build is not a broken site: leave the last good render
		// (and isReady) in place and don't count it as a failure.
		if ctxErr := ctx.Err(); ctxErr != nil {
			status = "canceled"
			return ctxErr
		}

		svc.isReady.Store(false)
		status = "fail"

		return fmt.Errorf("hugo build: %w", err)
	}

	syncer := fsync.NewSyncer()
	syncer.NoChmod = true
	syncer.NoTimes = true

	for _, lang := range []string{"ru", "en"} {
		// Sync modules folder
		glob := filepath.Join(svc.destDir, "public", lang, "modules/*")
		err = removeGlob(glob)
		if err != nil {
			return fmt.Errorf("clear %s: %w", svc.destDir, err)
		}

		oldLocation := filepath.Join(svc.baseDir, "public", lang, "modules")
		newLocation := filepath.Join(svc.destDir, "public", lang, "modules")
		err = syncDir(syncer, oldLocation, newLocation)
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", oldLocation, newLocation, err)
		}

		// Sync search index folder
		searchGlob := filepath.Join(svc.destDir, "public", lang, "search/*")
		err = removeGlob(searchGlob)
		if err != nil {
			return fmt.Errorf("clear %s: %w", svc.destDir, err)
		}

		searchOldLocation := filepath.Join(svc.baseDir, "public", lang, "search")
		searchNewLocation := filepath.Join(svc.destDir, "public", lang, "search")
		err = syncDir(syncer, searchOldLocation, searchNewLocation)
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", searchOldLocation, searchNewLocation, err)
		}
	}

	svc.isReady.Store(true)

	return nil
}

func (svc *Service) buildHugo(ctx context.Context) error {
	flags := &hugo.Flags{
		LogLevel: "debug",
		Source:   svc.baseDir,
		CfgDir:   filepath.Join(svc.baseDir, "config"),
	}

	svc.metrics.Grouped().ExpireGroupMetricByName(metrics.DocsBuilderModuleRenderErrorGroup, metrics.DocsBuilderModuleRenderError)

	for {
		// A single hugo pass runs to completion once started, but this
		// self-heal loop can rerun it once per broken module it strips —
		// several full-site builds in one call. Bail between passes so a
		// canceled request stops multiplying rebuilds instead of holding the
		// build slot for every remaining broken module.
		if err := ctx.Err(); err != nil {
			return err
		}

		buildErr := hugo.Build(ctx, flags, svc.logger)
		if buildErr == nil {
			return nil
		}

		if moduleName, ok := getModuleNameFromErrorPath(buildErr.Error()); ok {
			paths := []string{
				filepath.Join(svc.baseDir, contentDir, moduleName),
				filepath.Join(svc.baseDir, modulesDir, moduleName),
			}

			for _, path := range paths {
				err := os.RemoveAll(path)
				if err != nil {
					return fmt.Errorf("remove module: %w", err)
				}
			}

			err := svc.removeModuleFromChannelMapping(moduleName)
			if err != nil {
				return fmt.Errorf("remove module from channel mapping: %w", err)
			}

			svc.logger.Warn("removed broken module", slog.String("name", moduleName), log.Err(buildErr))
			svc.metrics.Grouped().GaugeSet(metrics.DocsBuilderModuleRenderErrorGroup, metrics.DocsBuilderModuleRenderError, 1, map[string]string{"module": moduleName})
			continue
		}

		return buildErr
	}
}

func (svc *Service) removeModuleFromChannelMapping(moduleName string) error {
	return svc.channelMappingEditor.edit(func(m channelMapping) {
		delete(m, moduleName)
	})
}

func getModuleNameFromErrorPath(errorMessage string) (string, bool) {
	match := assembleErrorRegexp.FindStringSubmatch(errorMessage)
	if len(match) == 6 {
		// return only module name
		return match[2], true
	}

	return "", false
}

func (svc *Service) parseModulePath(modulePath string) ( /*moduleName*/ string /*channel*/, string) {
	s := strings.Split(modulePath, "/")
	if len(s) < 2 {
		svc.logger.Error("failed to parse", slog.String("path", modulePath))
		return "", ""
	}

	return s[len(s)-2], s[len(s)-1]
}

// syncDir mirrors src into dst. If src does not exist, it is treated as an
// empty source and the sync is skipped: Hugo does not create an output
// directory for a language that has no module or search pages, and the
// preceding removeGlob has already cleared any stale content in dst. Erroring
// out on a missing source would fail the whole build in that legitimate case.
func syncDir(syncer *fsync.Syncer, src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("stat %s: %w", src, err)
	}

	return syncer.Sync(dst, src)
}

func removeGlob(path string) error {
	contents, err := filepath.Glob(path)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	for _, item := range contents {
		err = os.RemoveAll(item)
		if err != nil {
			return fmt.Errorf("remove all: %w", err)
		}
	}

	return nil
}
