/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetesversion

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type versionWatcher struct {
	ch          chan<- *semver.Version
	lastVersion *semver.Version
	watcher     *fsnotify.Watcher
	logger      *log.Logger
}

func (w *versionWatcher) watch(path string) error {
	var err error
	w.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new watcher: %w", err)
	}
	if err = w.watcher.Add(path); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	for {
		select {
		case _, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if err := w.handler(path); err != nil {
				return err
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}

func (w *versionWatcher) handler(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if len(content) == 0 {
		return nil
	}
	parsed, err := semver.NewVersion(strings.TrimSpace(string(content)))
	if err != nil {
		w.logger.Error("failed to parse version", "path", path, "content", string(content), log.Err(err))
		return fmt.Errorf("new version: %w", err)
	}
	if w.lastVersion == nil || !w.lastVersion.Equal(parsed) {
		w.lastVersion = parsed
		w.ch <- w.lastVersion
	}
	return nil
}
