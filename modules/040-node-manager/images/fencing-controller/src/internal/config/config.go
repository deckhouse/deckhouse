/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-controller/internal/common"
)

const disabledBindAddress = "0"

const (
	DefaultHealthProbeBindAddress = "0.0.0.0:4292"
	DefaultMetricsBindAddress     = "127.0.0.1:4291"
	DefaultLogLevel               = "info"
)

type Config struct {
	HealthProbeBindAddress string
	MetricsBindAddress     string

	LeaderElection          bool
	LeaderElectionNamespace string

	LogLevel string
}

func Load(args []string) (*Config, error) {
	cfg := &Config{
		LeaderElectionNamespace: strings.TrimSpace(os.Getenv(common.EnvPodNamespace)),
		LogLevel:                logLevelFromEnv(),
	}

	fs := flag.NewFlagSet(common.ComponentName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.HealthProbeBindAddress, "health-probe-bind-address", DefaultHealthProbeBindAddress,
		`address the healthz/readyz endpoints bind to, or "0" to disable them`)
	fs.StringVar(&cfg.MetricsBindAddress, "metrics-bind-address", DefaultMetricsBindAddress,
		`address the metrics endpoint binds to, or "0" to disable it`)
	fs.BoolVar(&cfg.LeaderElection, "leader-elect", false,
		"run only one active replica, arbitrated by a Lease in the pod namespace")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if err := validateBindAddress("--health-probe-bind-address", c.HealthProbeBindAddress); err != nil {
		return err
	}

	if err := validateBindAddress("--metrics-bind-address", c.MetricsBindAddress); err != nil {
		return err
	}

	if c.LeaderElection && c.LeaderElectionNamespace == "" {
		return fmt.Errorf("%s is empty while --leader-elect is set", common.EnvPodNamespace)
	}

	if _, err := log.ParseLevel(c.LogLevel); err != nil {
		return fmt.Errorf("%s=%q is invalid, must be one of trace/debug/info/warn/error/fatal", common.EnvLogLevel, c.LogLevel)
	}

	return nil
}

func validateBindAddress(name, addr string) error {
	if strings.TrimSpace(addr) == "" {
		return errors.New(name + " is empty")
	}

	if addr == disabledBindAddress {
		return nil
	}

	if _, _, err := net.SplitHostPort(addr); err != nil {
		return fmt.Errorf("%s=%q is not a host:port address: %w", name, addr, err)
	}

	return nil
}

func logLevelFromEnv() string {
	if level := strings.TrimSpace(os.Getenv(common.EnvLogLevel)); level != "" {
		return level
	}

	return DefaultLogLevel
}
