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
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
)

type versionWatcher struct {
	ch          chan<- *semver.Version
	lastVersion *semver.Version
	watcher     *fsnotify.Watcher
}

func (w *versionWatcher) watch(path string) (err error) {
	if err = waitForExisting(path); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != "" {
		parsed, err := semver.NewVersion(strings.TrimSpace(string(data)))
		if err != nil {
			return err
		}
		w.lastVersion = parsed
		w.ch <- parsed
	}

	if w.watcher, err = fsnotify.NewWatcher(); err != nil {
		return err
	}
	if err = w.watcher.Add(path); err != nil {
		return err
	}
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if data, err = os.ReadFile(path); err != nil {
				return err
			}
			if err = w.handler(string(data), event); err != nil {
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

func (w *versionWatcher) handler(content string, _ fsnotify.Event) error {
	parsed, err := semver.NewVersion(strings.TrimSpace(content))
	if err != nil {
		return err
	}
	if w.lastVersion == nil || !w.lastVersion.Equal(parsed) {
		w.lastVersion = parsed
		w.ch <- w.lastVersion
	}
	return nil
}

func waitForExisting(path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if os.IsNotExist(err) {
			time.Sleep(10 * time.Millisecond)
		} else {
			return err
		}
	}
}
