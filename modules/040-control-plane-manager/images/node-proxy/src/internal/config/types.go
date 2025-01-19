package config

type AuthMode int

const (
	AuthDev AuthMode = iota
	AuthCert
)
