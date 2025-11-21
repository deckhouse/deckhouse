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

type ConversionsStore struct {
	mtx sync.Mutex

	converters map[string]*Converter
}

func NewConversionsStore() *ConversionsStore {
	return &ConversionsStore{}
}

func (s *ConversionsStore) Add(module, pathToConversions string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.converters == nil {
		s.converters = make(map[string]*Converter)
	}

	converter, err := newConverter(pathToConversions)
	if err != nil {
		return err
	}

	s.converters[module] = converter

	return nil
}

func (s *ConversionsStore) Get(module string) *Converter {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if con, ok := s.converters[module]; ok {
		return con
	}
	return &Converter{latest: 1}
}
