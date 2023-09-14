package config

import (
	"fmt"
	"os"
)

// ScanInterval Scan block device interval seconds
const (
	ScanInterval   = 10
	NodeName       = "NODE_NAME"
	MetricsPortEnv = "METRICS_PORT"
)

type Options struct {
	ScanInterval int
	NodeName     string
	MetricsPort  string
}

func NewConfig() (*Options, error) {
	var opts Options
	opts.ScanInterval = ScanInterval

	opts.NodeName = os.Getenv(NodeName)
	if opts.NodeName == "" {
		return nil, fmt.Errorf("required NODE_NAME env variable is not specified")
	}

	opts.MetricsPort = os.Getenv(MetricsPortEnv)
	if opts.MetricsPort == "" {
		opts.MetricsPort = ":8080"
	}

	return &opts, nil
}
