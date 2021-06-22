package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util"
)

// NewTempStateCache creates new cache instance in tmp directory
func NewTempStateCache(identity string) (*StateCache, error) {
	cacheDir := filepath.Join(app.CacheDir, util.Sha256Encode(identity))
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

// SaveStruct saves bytes to a file
func (s *StateCache) Save(name string, content []byte) error {
	if err := ioutil.WriteFile(s.GetPath(name), content, 0o600); err != nil {
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
func (s *StateCache) InCache(name string) bool {
	info, err := os.Stat(s.GetPath(name))
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
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

func (s *StateCache) Delete(name string) {
	if s.InCache(name) {
		_ = os.Remove(s.GetPath(name))
	}
}

func (s *StateCache) Load(name string) []byte {
	content, err := ioutil.ReadFile(s.GetPath(name))
	if err != nil {
		log.ErrorLn(err.Error())
	}
	return content
}

// LoadStruct loads go struct from the cache
func (s *StateCache) LoadStruct(name string, v interface{}) error {
	d := s.Load(name)
	if d == nil {
		return fmt.Errorf("can't load struct")
	}

	return gob.NewDecoder(bytes.NewBuffer(d)).Decode(v)
}

func (s *StateCache) GetPath(name string) string {
	return filepath.Join(s.dir, name)
}

func (s *StateCache) Iterate(iterFunc func(string, []byte) error) error {
	walkFunc := func(path string, info os.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		content, err := ioutil.ReadFile(path)
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

// DummyCache is a cache implementation which saves nothing and nowhere
type DummyCache struct{}

func (d *DummyCache) Save(n string, c []byte) error            { return nil }
func (d *DummyCache) InCache(n string) bool                    { return false }
func (d *DummyCache) Clean()                                   {}
func (d *DummyCache) Delete(n string)                          {}
func (d *DummyCache) Load(n string) []byte                     { return nil }
func (d *DummyCache) LoadStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) SaveStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) GetPath(n string) string                  { return "" }
func (d *DummyCache) Iterate(func(string, []byte) error) error { return nil }
