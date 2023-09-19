package config

import (
	"os"
)

// ScanInterval Scan block device interval seconds
const (
	ScanInterval     = 10
	ConfigSecretName = "d8-sds-drbd-operator-config"
	NodeName         = "NODE_NAME"
	MetricsPortEnv   = "METRICS_PORT"
)

type Options struct {
	ScanInterval     int
	ConfigSecretName string
	MetricsPort      string
}

func NewConfig() (*Options, error) {
	var opts Options
	opts.ScanInterval = ScanInterval
	opts.ConfigSecretName = ConfigSecretName

	opts.MetricsPort = os.Getenv(MetricsPortEnv)
	if opts.MetricsPort == "" {
		opts.MetricsPort = ":8080"
	}

	return &opts, nil
}
