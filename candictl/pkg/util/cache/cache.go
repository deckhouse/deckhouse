package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
)

var once sync.Once

func encode(input string) string {
	// TODO: declare hasher once
	hasher := sha256.New()

	hasher.Write([]byte(input))

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

type Cache interface {
	Save(string, []byte)
	SaveStruct(string, interface{}) error

	Load(string) []byte
	LoadStruct(string, interface{}) error

	Delete(string)
	Clean()

	GetPath(string) string
	Iterate(func(string, []byte) error) error
	InCache(string) bool
}

var (
	_ Cache = &StateCache{}
	_ Cache = &DummyCache{}
)

var globalCache Cache = &DummyCache{}

func initCache(dir string) error {
	var err error
	once.Do(func() {
		globalCache, err = NewTempStateCache(dir)
	})
	return err
}

func Init(dir string) error {
	return initCache(dir)
}

func Global() Cache {
	return globalCache
}

type StateCache struct {
	dir string
}

// NewTempStateCache creates new cache instance in tmp directory
func NewTempStateCache(identity string) (*StateCache, error) {
	cacheDir := filepath.Join(app.CacheDir, encode(identity))
	return NewStateCache(cacheDir)
}

// NewTempStateCache creates new cache instance in specified directory
func NewStateCache(dir string) (*StateCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("can't create cache directory: %w", err)
	}

	_, err := os.Stat(filepath.Join(dir, ".tombstone"))
	if os.IsNotExist(err) {
		return &StateCache{dir: dir}, nil
	}

	return nil, fmt.Errorf("cache %s marked as exhausted", dir)
}

// SaveStruct saves bytes to a file
func (s *StateCache) Save(name string, content []byte) {
	if err := ioutil.WriteFile(s.GetPath(name), content, 0600); err != nil {
		log.ErrorF("Can't save terraform state in cache: %v", err)
	}
}

// SaveStruct saves go struct into the cache as a blob
func (s *StateCache) SaveStruct(name string, v interface{}) error {
	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(v)
	if err != nil {
		return err
	}

	s.Save(name, b.Bytes())
	return nil
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
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return
	}

	_, err := os.Create(filepath.Join(s.dir, ".tombstone"))
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

func (d *DummyCache) Save(n string, c []byte)                  {}
func (d *DummyCache) InCache(n string) bool                    { return false }
func (d *DummyCache) Clean()                                   {}
func (d *DummyCache) Delete(n string)                          {}
func (d *DummyCache) Load(n string) []byte                     { return nil }
func (d *DummyCache) LoadStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) SaveStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) GetPath(n string) string                  { return "" }
func (d *DummyCache) Iterate(func(string, []byte) error) error { return nil }
