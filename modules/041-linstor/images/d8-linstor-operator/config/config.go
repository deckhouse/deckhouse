package config

import (
	"os"
)

const (
	MetricsPortEnv = "METRICS_PORT"
	CertDirEnv     = "CERT_DIR"
	ScanInterval   = 10
)

type Options struct {
	MetricsPort  string
	CertDir      string
	ScanInterval int
	SCStable     struct {
		Replicas string
		Quorum   string
	}
	SCBad struct {
		Replicas string
		Quorum   string
	}
}

func NewConfig() (*Options, error) {
	var opts Options

	opts.MetricsPort = os.Getenv(MetricsPortEnv)
	if opts.MetricsPort == "" {
		opts.MetricsPort = ":8080"
	}

	opts.CertDir = os.Getenv(CertDirEnv)

	opts.ScanInterval = ScanInterval

	return &opts, nil
}
