/*
Copyright 2026 Flant JSC

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

// Package store notices the in-cluster cache's data left behind on a node.
//
// The blobs are kept on purpose when the cache is turned off: turning it back on then
// refills from what is already there instead of from scratch, and over a slow link that
// difference is hours rather than minutes. Deleting them would make the decision
// irreversible in the one direction that hurts.
//
// What must not happen is for them to be kept quietly. Nothing else in the module would
// ever mention the disk they occupy — the storage that wrote them is gone, and a node's
// free space is not something anybody re-examines — so the agent measures them and says
// so. It is the only component the module runs on every node.
package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Usage measures the data under a path.
//
// Returns zero when there is nothing there, which is the ordinary case: a node that never
// ran a storage replica has no such directory.
func Usage(path string) (int64, error) {
	if path == "" {
		return 0, nil
	}

	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return 0, nil
	case err != nil:
		return 0, err
	case !info.IsDir():
		return 0, nil
	}

	var total int64
	err = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// One unreadable subtree is not a reason to report nothing. The number
			// answers "is it worth reclaiming this space", and an underestimate serves
			// that better than an error does.
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil
		}
		// Regular files only. A symlink is counted where its target lives, if that is
		// even inside this tree, and counting it here as well would double it.
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})

	return total, err
}
