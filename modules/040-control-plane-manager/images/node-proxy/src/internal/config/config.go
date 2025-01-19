package config

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	AuthMode   AuthMode
	SocketPath string
	APIHosts   []string
	CertPath   string
	KeyPath    string
	CACertPath string
}

func ParseAuthMode(mode string) AuthMode {
	switch strings.ToLower(mode) {
	case "dev":
		return AuthDev
	case "cert":
		return AuthCert
	default:
		log.Infoln("Cert authorization")
		return AuthCert
	}
}
