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
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/fsnotify/fsnotify"
)

type versionWatcher struct {
	ch          chan<- *semver.Version
	lastVersion *semver.Version
	watcher     *fsnotify.Watcher
	logger      logger.Logger
}

func (w *versionWatcher) watch(path string) (err error) {
	if w.watcher, err = fsnotify.NewWatcher(); err != nil {
		return err
	}
	if err = w.watcher.Add(path); err != nil {
		return err
	}
	for {
		select {
		case _, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if err = w.handler(path); err != nil {
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
		return err
	}
	if len(content) == 0 {
		return nil
	}
	parsed, err := semver.NewVersion(strings.TrimSpace(string(content)))
	if err != nil {
		w.logger.Error("failed to parse version", "path", path, "content", string(content), "err", err)
		return err
	}
	if w.lastVersion == nil || !w.lastVersion.Equal(parsed) {
		w.lastVersion = parsed
		w.ch <- w.lastVersion
	}
	return nil
}
