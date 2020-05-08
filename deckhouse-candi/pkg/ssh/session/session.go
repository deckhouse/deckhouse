package session

import (
	"fmt"
)

// TODO rename to Settings
// Session is used to store ssh settings
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
	AuthSock string
}

func NewSession() *Session {
	return &Session{}
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
