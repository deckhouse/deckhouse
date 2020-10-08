package session

import (
	"fmt"
	"strings"
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
	BastionPort string
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

func (s *Session) String() string {
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
		builder.WriteString(fmt.Sprintf("%s@%s", s.User, s.Host))
	} else {
		builder.WriteString(s.Host)
	}

	if s.Port != "" {
		builder.WriteString(fmt.Sprintf(":%s", s.Port))
	}

	return builder.String()
}

func (s *Session) Copy() *Session {
	ses := &Session{}

	ses.Port = s.Port
	ses.Host = s.Host
	ses.User = s.User
	ses.BastionHost = s.BastionHost
	ses.BastionPort = s.BastionPort
	ses.BastionUser = s.BastionUser
	ses.ExtraArgs = s.ExtraArgs
	ses.AuthSock = s.AuthSock

	ses.PrivateKeys = append(ses.PrivateKeys, s.PrivateKeys...)
	return ses
}
