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

func (svc *Service) Build() error {
	start := time.Now()
	status := "ok"
	defer func() {
		dur := time.Since(start).Seconds()
		svc.metrics.CounterAdd(metrics.DocsBuilderBuildTotal, 1, map[string]string{"status": status})
		svc.metrics.HistogramObserve(metrics.DocsBuilderBuildDurationSeconds, dur, map[string]string{"status": status}, nil)
	}()

	err := svc.buildHugo()
	if err != nil {
		svc.isReady.Store(false)
		status = "fail"

		return fmt.Errorf("hugo build: %w", err)
	}

	for _, lang := range []string{"ru", "en"} {
		// Sync modules folder
		glob := filepath.Join(svc.destDir, "public", lang, "modules/*")
		err = removeGlob(glob)
		if err != nil {
			return fmt.Errorf("clear %s: %w", svc.destDir, err)
		}

		syncer := fsync.NewSyncer()
		syncer.NoChmod = true
		syncer.NoTimes = true

		oldLocation := filepath.Join(svc.baseDir, "public", lang, "modules")
		newLocation := filepath.Join(svc.destDir, "public", lang, "modules")
		err = syncer.Sync(newLocation, oldLocation)
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
		err = syncer.Sync(searchNewLocation, searchOldLocation)
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", searchOldLocation, searchNewLocation, err)
		}
	}

	svc.isReady.Store(true)

	return nil
}

func (svc *Service) buildHugo() error {
	flags := &hugo.Flags{
		LogLevel: "debug",
		Source:   svc.baseDir,
		CfgDir:   filepath.Join(svc.baseDir, "config"),
	}

	for {
		buildErr := hugo.Build(flags, svc.logger)
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
