// Copyright 2021 Flant JSC
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

package fs

import (
	"github.com/fsnotify/fsnotify"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func StartFileWatcher(path string, fsEventHanlder func(event fsnotify.Event), done chan struct{}, logger log.Logger) (watcher *fsnotify.Watcher, err error) {
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(path)
	if err != nil {
		return nil, err
	}

	logger.LogInfoF("Start watcher for file %s\n", path)

	go func() {
		defer close(done)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					// watcher.Close() was called
					return
				}
				if fsEventHanlder != nil {
					fsEventHanlder(event)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					// r.stateWatcher.Close() was called
					return
				}
				logger.LogWarnF("fs watcher: %v\n", err.Error())
			}
		}
	}()

	return watcher, nil
}
