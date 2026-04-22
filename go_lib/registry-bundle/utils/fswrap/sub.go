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

package fswrap

import (
	"io/fs"
	"path"
)

var (
	_ fs.ReadDirFS = (*SubFS)(nil)
	_ fs.SubFS     = (*SubFS)(nil)
)

// NewSubFS wraps fsys so paths are resolved under a virtual "." root and [fs.SubFS.Sub] works
// (needed for some archive-backed [io/fs.FS] implementations).
func NewSubFS(fsys fs.FS) *SubFS {
	return &SubFS{base: fsys, relRoot: "."}
}

// SubFS restricts access to base using relRoot as the current directory prefix (slash-separated).
type SubFS struct {
	base    fs.FS
	relRoot string
}

func (s *SubFS) Open(name string) (fs.File, error) {
	return s.base.Open(s.join(name))
}

func (s *SubFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if rdFS, ok := s.base.(fs.ReadDirFS); ok {
		return rdFS.ReadDir(s.join(name))
	}
	return nil, fs.ErrInvalid
}

func (s *SubFS) Sub(dir string) (fs.FS, error) {
	dir = path.Clean(dir)
	if dir == "." {
		return s, nil
	}
	next := &SubFS{
		base:    s.base,
		relRoot: s.join(dir),
	}
	if _, err := next.Open("."); err != nil {
		return nil, err
	}
	return next, nil
}

func (s *SubFS) join(name string) string {
	return path.Join(s.relRoot, path.Clean(name))
}
