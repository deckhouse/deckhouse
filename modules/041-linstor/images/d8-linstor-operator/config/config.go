package config

import (
	"fmt"
	"os"
)

const (
	MetricsPortEnv = "METRICS_PORT"

	SCStableReplicas = "SC_STABLE_REPLICAS"
	SCStableQuorum   = "SC_STABLE_QUORUM"

	SCBadReplicas = "SC_BAD_REPICAS"
	SCBadQuorum   = "SC_BAD_QUORUM"
)

type Options struct {
	MetricsPort string
	SCStable    struct {
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

	opts.SCStable.Replicas = os.Getenv(SCStableReplicas)
	if opts.SCStable.Replicas == "" {
		return nil, fmt.Errorf("required %s env variable is not set", SCStableReplicas)
	}
	opts.SCStable.Quorum = os.Getenv(SCStableQuorum)
	if opts.SCStable.Quorum == "" {
		return nil, fmt.Errorf("required %s env variable is not set", SCStableQuorum)
	}

	opts.SCBad.Replicas = os.Getenv(SCBadReplicas)
	if opts.SCBad.Replicas == "" {
		return nil, fmt.Errorf("required %s env variable is not set", SCBadReplicas)
	}

	opts.SCBad.Quorum = os.Getenv(SCBadQuorum)
	if opts.SCBad.Quorum == "" {
		return nil, fmt.Errorf("required %s env variable is not set", SCBadQuorum)
	}

	opts.MetricsPort = os.Getenv(MetricsPortEnv)
	if opts.MetricsPort == "" {
		opts.MetricsPort = ":8080"
	}

	return &opts, nil
}
