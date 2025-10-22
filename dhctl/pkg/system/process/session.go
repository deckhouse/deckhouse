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

package process

import (
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var DefaultSession *Session

func init() {
	DefaultSession = NewSession()
}

type Stopable interface {
	Stop()
}

type Session struct {
	Stopables []Stopable
}

func NewSession() *Session {
	return &Session{
		Stopables: make([]Stopable, 0),
	}
}

func (s *Session) Stop() {
	if s == nil {
		return
	}
	var wg sync.WaitGroup
	count := 0
	for _, stopable := range s.Stopables {
		if stopable == nil {
			continue
		}
		wg.Add(1)
		count++
		go func(s Stopable) {
			defer wg.Done()
			s.Stop()
		}(stopable)
	}
	log.DebugF("Wait while %d processes stops\n", count)
	wg.Wait()
}

func (s *Session) RegisterStoppable(stopable Stopable) {
	s.Stopables = append(s.Stopables, stopable)
}
