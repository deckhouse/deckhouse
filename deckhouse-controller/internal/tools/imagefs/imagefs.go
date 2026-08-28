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

// Package imagefs holds the path handling shared by the tar extractors that unpack
// module and package images onto the filesystem.
package imagefs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeJoin resolves a path taken from an image archive against root and rejects one that
// escapes it. CWE-22 check: every archive-controlled path - an entry name as well as a link
// target - has to go through it, because both end up as arguments to filesystem calls.
func SafeJoin(root, name string) (string, error) {
	target := filepath.Join(root, name)

	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal detected in the archive: malicious path %v", name)
	}

	return target, nil
}
