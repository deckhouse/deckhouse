package cache

import (
	"bytes"
	"encoding/base32"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
)

var once sync.Once

type Cache interface {
	Save(string, []byte)
	SaveByPath(string, string)
	InCache(string) bool
	AddToClean(string)
	Clean()
	Delete(string)
	Load(string) []byte
	LoadStruct(string, interface{}) error
	SaveStruct(string, interface{}) error
	ObjectPath(string) string
	GetDir() string
}

type StateCache struct {
	dir          string
	stateToClean []string
}

var globalCache Cache = &DummyCache{}

func initCache(dir string) error {
	var err error
	once.Do(func() {
		globalCache, err = NewStateCache(dir)
	})
	return err
}

func Global() Cache {
	return globalCache
}

var (
	_ Cache = &StateCache{}
	_ Cache = &DummyCache{}
)

func NewStateCache(dir string) (*StateCache, error) {
	_ = os.MkdirAll(dir, 0755)
	_, err := os.Stat(filepath.Join(dir, ".tombstone"))
	if os.IsNotExist(err) {
		return &StateCache{dir: dir}, nil
	}
	return nil, fmt.Errorf("cache %s marked as exhausted", dir)
}

func (s *StateCache) Save(name string, content []byte) {
	if err := ioutil.WriteFile(s.ObjectPath(name), content, 0755); err != nil {
		log.ErrorF("Can't save terraform state in cache: %v", err)
	}
}

func (s *StateCache) SaveByPath(name, path string) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.ErrorF("Can't load terraform state in cache: %v", err)
		return
	}

	s.Save(name, content)
}

func (s *StateCache) InCache(name string) bool {
	info, err := os.Stat(s.ObjectPath(name))
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func (s *StateCache) AddToClean(name string) {
	s.stateToClean = append(s.stateToClean, s.ObjectPath(name))
}

func (s *StateCache) Clean() {
	for _, state := range s.stateToClean {
		_ = os.Remove(state)
	}
	_, err := os.Create(filepath.Join(s.dir, ".tombstone"))
	if err != nil {
		log.Warning("Can't mark the cache as exhausted ...\n")
	}
}

func (s *StateCache) Delete(name string) {
	if s.InCache(name) {
		_ = os.Remove(s.ObjectPath(name))
	}
}

func (s *StateCache) Load(name string) []byte {
	content, err := ioutil.ReadFile(s.ObjectPath(name))
	if err != nil {
		log.ErrorLn(err.Error())
	}
	return content
}

func (s *StateCache) LoadStruct(name string, v interface{}) error {
	d := s.Load(name)
	if d == nil {
		return nil
	}
	return unmarshal(d, v)
}

func (s *StateCache) SaveStruct(name string, v interface{}) error {
	d, err := marshal(v)
	if err != nil {
		return err
	}
	s.Save(name, d)
	return nil
}

func (s *StateCache) ObjectPath(name string) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s.tfstate", name))
}

func (s *StateCache) GetDir() string {
	return s.dir
}

func Init(dir string) error {
	return initCache(filepath.Join(app.TerraformStateDir, encode(dir)))
}

var encoding = base32.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZ134567")

func encode(input string) string {
	toDecodeString := []byte(input)
	return strings.TrimRight(encoding.EncodeToString(fnv.New32().Sum(toDecodeString)), "=")
}

func marshal(v interface{}) ([]byte, error) {
	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(v)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func unmarshal(data []byte, v interface{}) error {
	b := bytes.NewBuffer(data)
	return gob.NewDecoder(b).Decode(v)
}

func RemoveEverythingFromCache(cacheDir string) {
	cacheDir = filepath.Join(app.TerraformStateDir, encode(cacheDir))
	dir, err := ioutil.ReadDir(cacheDir)
	if err != nil {
		log.ErrorLn(err)
		return
	}
	for _, d := range dir {
		_ = os.RemoveAll(filepath.Join([]string{cacheDir, d.Name()}...))
	}
}

type DummyCache struct{}

func (d *DummyCache) Save(n string, c []byte)                  {}
func (d *DummyCache) SaveByPath(n string, k string)            {}
func (d *DummyCache) InCache(n string) bool                    { return false }
func (d *DummyCache) AddToClean(n string)                      {}
func (d *DummyCache) Clean()                                   {}
func (d *DummyCache) Delete(n string)                          {}
func (d *DummyCache) Load(n string) []byte                     { return nil }
func (d *DummyCache) LoadStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) SaveStruct(n string, v interface{}) error { return nil }
func (d *DummyCache) ObjectPath(n string) string               { return "" }
func (d *DummyCache) GetDir() string                           { return "" }
