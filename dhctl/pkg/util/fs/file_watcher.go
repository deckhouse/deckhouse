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
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

func StartFileWatcher(ctx context.Context, path string, fsEventHanlder func(event fsnotify.Event), done chan struct{}) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(path)
	if err != nil {
		return nil, err
	}

	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Start watcher for file %s", path))

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
				dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("fs watcher: %v", err.Error()))
			}
		}
	}()

	return watcher, nil
}
