package session

import (
	"fmt"
)

type Stopable interface {
	Stop()
}

// Session is used store ssh settings
type Session struct {
	// input
	PrivateKeys []string
	Host        string
	User        string
	Port        string
	BastionHost string
	BastionUser string
	ExtraArgs   string

	// runtime
	AuthSock  string
	stopables []Stopable
}

func NewSession() *Session {
	return &Session{
		stopables: make([]Stopable, 0),
	}
}

func (s *Session) Stop() error {
	if s == nil {
		return nil
	}
	for _, st := range s.stopables {
		st.Stop()
	}
	return nil
}

func (s *Session) RegisterStoppable(stopable Stopable) {
	// s.stopables = append(s.stopables, stopable)
}

// RemoteAddress returns host or username@host
func (s *Session) RemoteAddress() string {
	addr := s.Host
	if s.User != "" {
		addr = s.User + "@" + addr
	}
	return addr
}

func (s *Session) AuthSockEnv() string {
	if s.AuthSock != "" {
		return fmt.Sprintf("SSH_AUTH_SOCK=%s", s.AuthSock)
	}
	return ""
}
