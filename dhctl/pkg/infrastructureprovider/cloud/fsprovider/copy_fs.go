// Copyright 2025 Flant JSC
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

package fsprovider

import (
	"io"
	"io/fs"
	"os"
	"path"
)

// todo copied from 1.25 go when we move to go 1.25 we need to use os.CopyFs with ReadLinkFS interface
func copyFS(dir string, fsys fs.FS, from string) error {
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}
		newPath := joinPath(dir, p)

		switch d.Type() {
		case os.ModeDir:
			return os.MkdirAll(newPath, 0777)
		case os.ModeSymlink:
			target, err := os.Readlink(path.Join(from, p))
			if err != nil {
				return err
			}
			return os.Symlink(target, newPath)
		case 0:
			r, err := fsys.Open(p)
			if err != nil {
				return err
			}
			defer r.Close()
			info, err := r.Stat()
			if err != nil {
				return err
			}
			w, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666|info.Mode()&0777)
			if err != nil {
				return err
			}

			if _, err := io.Copy(w, r); err != nil {
				w.Close()
				return &os.PathError{Op: "Copy", Path: newPath, Err: err}
			}
			return w.Close()
		default:
			return &os.PathError{Op: "CopyFS", Path: p, Err: os.ErrInvalid}
		}
	})
}

func joinPath(dir, name string) string {
	if len(dir) > 0 && os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + name
	}
	return path.Join(dir, name)
}
