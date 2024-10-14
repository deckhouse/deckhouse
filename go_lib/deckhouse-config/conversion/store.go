/*
Copyright 2024 Flant JSC

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

package conversion

import "sync"

var (
	instance *ConversionsStore
	once     sync.Once
)

type ConversionsStore struct {
	mtx sync.Mutex

	converters map[string]*Converter
}

func Store() *ConversionsStore {
	once.Do(func() {
		instance = &ConversionsStore{}
	})
	return instance
}

func (s *ConversionsStore) Add(module, pathToConversions string) (err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.converters == nil {
		s.converters = make(map[string]*Converter)
	}
	s.converters[module], err = newConverter(pathToConversions)
	return err
}

func (s *ConversionsStore) Get(module string) *Converter {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if con, ok := s.converters[module]; ok {
		return con
	}
	return &Converter{latest: 1}
}
