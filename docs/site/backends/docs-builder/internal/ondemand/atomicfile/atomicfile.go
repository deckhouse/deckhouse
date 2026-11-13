// Copyright 2026 Flant JSC
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

// Package atomicfile writes the files of the cache volume nginx serves: a
// file is replaced at once, never read half written.
package atomicfile

import (
	"io"
	"os"
	"path/filepath"
)

// Write replaces path at once with what write writes.
func Write(path string, write func(io.Writer) error) error {
	tmp, err := Temp(path, write)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}

// Temp writes a temporary file next to path and returns its name.
func Temp(path string, write func(io.Writer) error) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-")
	if err != nil {
		return "", err
	}

	if err := write(f); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}

	// Served by nginx from another container.
	if err := os.Chmod(f.Name(), 0644); err != nil {
		os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}
