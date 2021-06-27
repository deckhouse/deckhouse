// Copyright 2021 Flant CJSC
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

package session

import (
	"fmt"
	"strings"
	"sync"
)

type Input struct {
	PrivateKeys    []string
	User           string
	Port           string
	BastionHost    string
	BastionPort    string
	BastionUser    string
	ExtraArgs      string
	AvailableHosts []string
}

// TODO rename to Settings
// Session is used to store ssh settings
type Session struct {
	// input
	PrivateKeys []string
	User        string
	Port        string
	BastionHost string
	BastionPort string
	BastionUser string
	ExtraArgs   string

	// runtime
	AuthSock string

	lock           sync.RWMutex
	host           string
	availableHosts []string
	remainingHosts []string
}

func NewSession(input Input) *Session {
	s := &Session{
		PrivateKeys: input.PrivateKeys,
		User:        input.User,
		Port:        input.Port,
		BastionHost: input.BastionHost,
		BastionPort: input.BastionPort,
		BastionUser: input.BastionUser,
		ExtraArgs:   input.ExtraArgs,
	}

	s.SetAvailableHosts(input.AvailableHosts)

	return s
}

func (s *Session) Host() string {
	defer s.lock.RUnlock()
	s.lock.RLock()
	return s.host
}

// ChoiceNewHost choice new host for connection
func (s *Session) ChoiceNewHost() {
	defer s.lock.Unlock()
	s.lock.Lock()

	s.selectNewHost()
}

func (s *Session) SetAvailableHosts(hosts []string) {
	defer s.lock.Unlock()
	s.lock.Lock()

	s.availableHosts = make([]string, len(hosts))
	copy(s.availableHosts, hosts)

	s.resetUsedHosts()
	s.selectNewHost()
}

func (s *Session) CountHosts() int {
	defer s.lock.RUnlock()
	s.lock.RLock()

	return len(s.availableHosts)
}

// RemoteAddress returns host or username@host
func (s *Session) RemoteAddress() string {
	defer s.lock.RUnlock()
	s.lock.RLock()

	addr := s.host
	if s.User != "" {
		addr = s.User + "@" + addr
	}
	return addr
}

func (s *Session) AuthSockEnv() string {
	defer s.lock.RUnlock()
	s.lock.RLock()

	if s.AuthSock != "" {
		return fmt.Sprintf("SSH_AUTH_SOCK=%s", s.AuthSock)
	}
	return ""
}

func (s *Session) String() string {
	defer s.lock.RUnlock()
	s.lock.RLock()

	builder := strings.Builder{}
	builder.WriteString("ssh ")

	if s.BastionHost != "" {
		builder.WriteString("-J ")
		if s.BastionUser != "" {
			builder.WriteString(fmt.Sprintf("%s@%s", s.BastionUser, s.BastionHost))
		} else {
			builder.WriteString(s.BastionHost)
		}
		if s.BastionPort != "" {
			builder.WriteString(fmt.Sprintf(":%s", s.BastionPort))
		}
		builder.WriteString(" ")
	}

	if s.User != "" {
		builder.WriteString(fmt.Sprintf("%s@%s", s.User, s.host))
	} else {
		builder.WriteString(s.host)
	}

	if s.Port != "" {
		builder.WriteString(fmt.Sprintf(":%s", s.Port))
	}

	return builder.String()
}

func (s *Session) Copy() *Session {
	defer s.lock.RUnlock()
	s.lock.RLock()

	ses := &Session{}

	ses.Port = s.Port
	ses.User = s.User
	ses.BastionHost = s.BastionHost
	ses.BastionPort = s.BastionPort
	ses.BastionUser = s.BastionUser
	ses.ExtraArgs = s.ExtraArgs
	ses.AuthSock = s.AuthSock
	ses.host = s.host

	ses.PrivateKeys = append(ses.PrivateKeys, s.PrivateKeys...)

	ses.availableHosts = make([]string, len(s.availableHosts))
	copy(ses.availableHosts, s.availableHosts)

	ses.resetUsedHosts()

	return ses
}

// resetUsedHosts if all available host is used this function reset
func (s *Session) resetUsedHosts() {
	s.remainingHosts = make([]string, len(s.availableHosts))
	copy(s.remainingHosts, s.availableHosts)
}

func (s *Session) selectNewHost() {
	if len(s.availableHosts) == 0 {
		s.host = ""
		return
	}

	if len(s.remainingHosts) == 0 {
		s.resetUsedHosts()
	}

	indx := 0
	host := s.remainingHosts[indx]
	s.remainingHosts = append(s.remainingHosts[:indx], s.remainingHosts[indx+1:]...)

	s.host = host
}
