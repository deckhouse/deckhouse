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

package node

import (
	"context"
	"sync"
	"time"
)

var (
	sudoSessionMutex sync.Mutex
	sudoSessions     = make(map[string]*SudoSession)
)

type SudoSession struct {
	Host      string
	Validated bool
	ValidTime time.Time
	mutex     sync.Mutex
}

func GetSudoSession(host string) *SudoSession {
	sudoSessionMutex.Lock()
	defer sudoSessionMutex.Unlock()

	session, exists := sudoSessions[host]
	if !exists {
		session = &SudoSession{
			Host:      host,
			Validated: false,
		}
		sudoSessions[host] = session
	}

	return session
}

func (s *SudoSession) IsValid() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.Validated {
		return false
	}

	return time.Since(s.ValidTime) < 5*time.Minute
}

func (s *SudoSession) MarkValid() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Validated = true
	s.ValidTime = time.Now()
}

func (s *SudoSession) Invalidate() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Validated = false
}

func (s *SudoSession) NeedsSudoValidation(ctx context.Context, nodeInterface Interface) bool {
	if s.IsValid() {
		return false
	}

	cmd := nodeInterface.Command("sudo", "-n", "true")
	err := cmd.Run(ctx)
	if err == nil {
		s.MarkValid()
		return false
	}

	return true
}
