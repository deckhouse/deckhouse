// Copyright 2024 Flant JSC
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

package cache

import (
	"io"
	"os"
	"path/filepath"

	"github.com/google/renameio"
)

func (c *Cache) copyPackage(digest string, reader io.Reader) error {
	path := c.digestToPath(digest)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	file, err := renameio.TempFile("", path)
	if err != nil {
		return err
	}
	defer file.Cleanup()

	_, err = io.Copy(file, reader)
	if err != nil {
		return err
	}

	return file.CloseAtomicallyReplace()
}

func (c *Cache) digestToPath(digest string) string {
	// digest format is sha256:1234567...
	hash := digest[7:]
	return filepath.Join(c.root, "packages", hash[:2], hash)
}
