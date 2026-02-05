/*
Copyright 2023 Flant JSC

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

package watcher

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

// Run watches the given path and calls onChange when the file is updated
// (e.g. k8s configmap uses symlinks: old file is removed and a new link is created).
// It blocks until a fatal error occurs.
func Run(path string, onChange func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	if err := w.Add(path); err != nil {
		return err
	}

	log.Printf("start watching config changes at %s", path)
	for {
		select {
		case event := <-w.Events:
			if event.Op == fsnotify.Remove {
				// k8s configmaps use symlinks,
				// old file is deleted and a new link with the same name is created
				_ = w.Remove(event.Name)
				if err := w.Add(event.Name); err != nil {
					return err
				}
				if event.Name == path {
					onChange()
				}
			}
		case err := <-w.Errors:
			log.Printf("watch files error: %s\n", err)
		}
	}
}
