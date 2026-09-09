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

package fill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UploadInProgress reports whether somebody is pushing into this store right now.
//
// The one writer the syncer cannot serialise itself with. Inside the pod it is the only thing that
// writes to the cache, so a collection and a fill simply take turns — but the publication endpoint is
// reachable from outside, and what comes through it is an operator pushing a bundle. Reclaiming blobs
// underneath such a push would delete what it has uploaded but not yet referenced by a manifest.
//
// Read off the store's files, because that is where an upload in progress is: distribution keeps each
// one in `_uploads/<uuid>` until the manifest that references it lands. The publication endpoint being
// OPEN says nothing — it may stay open for the life of the cluster while pushes happen twice a year —
// so what is asked is whether anything is being written, not whether anything could be.
//
// Freshness is the second half of the question and not a refinement: an abandoned upload leaves its
// directory behind until distribution's own purge gets to it, days later, and treating that as an
// active push would mean a store that is never reclaimed again. So an upload counts as active only
// while it is still being touched.
func UploadInProgress(root string, within time.Duration, now time.Time) (bool, error) {
	repositories := filepath.Join(root, "docker", "registry", "v2", "repositories")

	if _, err := os.Stat(repositories); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("reading the store at %s: %w", repositories, err)
	}

	active := false

	err := filepath.WalkDir(repositories, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !uploadPath(repositories, path) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			// Vanished between the walk and the question, which for an upload means it has just
			// been finished or abandoned. Either way it is not evidence of one in progress.
			return nil //nolint:nilerr // a file that disappeared is not an error here
		}
		if now.Sub(info.ModTime()) <= within {
			active = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("walking the store at %s: %w", repositories, err)
	}

	return active, nil
}

// uploadPath reports whether a file belongs to an upload in progress, which distribution lays out as
// `<repositories>/<repository>/_uploads/<uuid>/<data|startedat|hashstates/...>`.
//
// Matched as a whole path segment, so that a repository merely named like the marker is not mistaken
// for one.
func uploadPath(repositories, path string) bool {
	relative, err := filepath.Rel(repositories, path)
	if err != nil {
		return false
	}

	for _, segment := range strings.Split(filepath.ToSlash(relative), "/") {
		if segment == uploadsDir {
			return true
		}
	}
	return false
}

// uploadsDir is where distribution keeps an upload until the manifest referencing it lands.
const uploadsDir = "_uploads"
