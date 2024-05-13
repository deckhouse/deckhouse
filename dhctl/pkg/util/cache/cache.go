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

package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

// NewTempStateCache creates new cache instance in tmp directory
func NewTempStateCache(identity string) (*StateCache, error) {
	cacheDir := filepath.Join(app.CacheDir, stringsutil.Sha256Encode(identity))
	return NewStateCache(cacheDir)
}

type StateCache struct {
	dir string
}

// NewTempStateCache creates new cache instance in specified directory
func NewStateCache(dir string) (*StateCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("can't create cache directory: %w", err)
	}

	_, err := os.Stat(filepath.Join(dir, ".tombstone"))
	if os.IsNotExist(err) {
		return &StateCache{dir: dir}, nil
	}

	return nil, fmt.Errorf("cache %s marked as exhausted", dir)
}

// NewStateCacheWithInitialState creates new cache instance in specified directory with initial state
func NewStateCacheWithInitialState(dir string, initialState map[string][]byte) (*StateCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("can't create cache directory: %w", err)
	}

	// prepare dir to be fresh for given initial state
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(p); err != nil {
			return nil, fmt.Errorf("unable to remove %s: %w", p, err)
		}
	}

	for filename, content := range initialState {
		p := filepath.Join(dir, filename)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			return nil, fmt.Errorf("error writing %s: %w", p, err)
		}
	}

	return &StateCache{dir: dir}, nil
}

// SaveStruct saves bytes to a file
func (s *StateCache) Save(name string, content []byte) error {
	if err := os.WriteFile(s.GetPath(name), content, 0o600); err != nil {
		log.ErrorF("Can't save terraform state in cache: %v", err)
	}

	return nil
}

// SaveStruct saves go struct into the cache as a blob
func (s *StateCache) SaveStruct(name string, v interface{}) error {
	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(v)
	if err != nil {
		return err
	}

	return s.Save(name, b.Bytes())
}

// InCache checks is file in cache or not
func (s *StateCache) InCache(name string) (bool, error) {
	info, err := os.Stat(s.GetPath(name))
	if os.IsNotExist(err) {
		return false, nil
	}
	return !info.IsDir(), nil
}

func (s *StateCache) Clean() {
	_ = os.RemoveAll(s.dir)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}

	_, err := os.Create(filepath.Join(s.dir, state.TombstoneKey))
	if err != nil {
		log.WarnF("Can't mark the cache as exhausted: %s ...\n", err)
	}
}

func (s *StateCache) CleanWithExceptions(excludeKeys ...string) {
	excludeKeysSet := map[string]struct{}{}
	for _, k := range excludeKeys {
		excludeKeysSet[k] = struct{}{}
	}

	keysToRemove := make([]string, 0)
	err := s.Iterate(func(key string, i []byte) error {
		if _, ok := excludeKeysSet[key]; ok {
			return nil
		}
		keysToRemove = append(keysToRemove, key)
		return nil
	})
	if err != nil {
		log.WarnF("Can't getting keys to remove: %s ...\n", err)
		return
	}

	// yes first write tombstone that is idempotent
	_, err = os.Create(filepath.Join(s.dir, state.TombstoneKey))
	if err != nil {
		log.WarnF("Can't mark the cache as exhausted: %s ...\n", err)
	}

	for _, key := range keysToRemove {
		s.Delete(key)
	}
}

func (s *StateCache) Delete(name string) {
	ok, _ := s.InCache(name)
	if ok {
		_ = os.Remove(s.GetPath(name))
	}
}

func (s *StateCache) Load(name string) ([]byte, error) {
	return os.ReadFile(s.GetPath(name))
}

// LoadStruct loads go struct from the cache
func (s *StateCache) LoadStruct(name string, v interface{}) error {
	d, err := s.Load(name)
	if err != nil {
		return fmt.Errorf("can't load struct for key %s: %v", name, err)
	}

	return gob.NewDecoder(bytes.NewBuffer(d)).Decode(v)
}

func (s *StateCache) GetPath(name string) string {
	if s == nil {
		return ""
	}
	return filepath.Join(s.dir, name)
}

func (s *StateCache) Iterate(iterFunc func(string, []byte) error) error {
	walkFunc := func(path string, info os.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("can't read file %s: %w", path, err)
		}

		return iterFunc(info.Name(), content)
	}

	if err := filepath.Walk(s.dir, walkFunc); err != nil {
		return fmt.Errorf("can't iterate the cache: %w", err)
	}
	return nil
}

func (s *StateCache) NeedIntermediateSave() bool {
	// cache use one file with terraform
	return false
}

// DummyCache is a cache implementation which saves nothing and nowhere
type DummyCache struct{}

func (d *DummyCache) Save(n string, c []byte) error            { return nil }
func (d *DummyCache) InCache(n string) (bool, error)           { return false, nil }
func (d *DummyCache) Clean()                                   {}
func (d *DummyCache) CleanWithExceptions(e ...string)          {}
func (d *DummyCache) Delete(n string)                          {}
func (d *DummyCache) Load(n string) ([]byte, error)            { return nil, nil }
func (d *DummyCache) LoadStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) SaveStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) GetPath(n string) string                  { return "" }
func (d *DummyCache) Iterate(func(string, []byte) error) error { return nil }
func (d *DummyCache) NeedIntermediateSave() bool               { return false }
