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
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

type Config struct {
	NodeName       string `env:"NODE_NAME" env-required:"true"`
	NodeUID        string
	NodeGroup      string `env:"NODE_GROUP" env-required:"true"`
	ProfileRefName string `env:"PROFILE_REF_NAME" env-required:"true"`

	WatchdogDevice       string        `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	WatchdogFeedInterval time.Duration `env:"WATCHDOG_FEED_INTERVAL" env-default:"1s"`
	WatchdogTimeout      time.Duration `env:"WATCHDOG_TIMEOUT" env-default:"10s"`

	FallbackHeartbeat time.Duration `env:"FALLBACK_HEARTBEAT" env-default:"1s"`
	FallbackTTL       time.Duration `env:"FALLBACK_TTL" env-default:"4s"`
	EvacuationDelay   time.Duration `env:"EVACUATION_DELAY" env-default:"6s"`

	RejoinInterval    time.Duration `env:"REJOIN_INTERVAL" env-default:"1s"`
	RejoinMaxInterval time.Duration `env:"REJOIN_MAX_INTERVAL" env-default:"10s"`

	KubernetesAPITimeout time.Duration `env:"KUBERNETES_API_TIMEOUT" env-default:"2s"`

	APISocketPath          string `env:"API_SOCKET_PATH" env-default:"/var/run/fencing-agent/fencing-agent.sock"`
	HealthProbeBindAddress string `env:"HEALTH_PROBE_BIND_ADDRESS" env-default:":8081"`
	LogLevel               string `env:"LOG_LEVEL" env-default:"info"`
}

func (c *Config) MustLoad() {
	if err := cleanenv.ReadEnv(c); err != nil {
		panic(err)
	}

	if err := c.validate(); err != nil {
		panic(err)
	}
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.NodeName) == "" {
		return errors.New("NODE_NAME is empty")
	}

	if strings.TrimSpace(c.NodeGroup) == "" {
		return errors.New("NODE_GROUP is empty")
	}

	if !slices.Contains(v1alpha1.ProfileNames(), v1alpha1.ProfileName(c.ProfileRefName)) {
		return fmt.Errorf("PROFILE_REF_NAME=%q is invalid, must be one of %v", c.ProfileRefName, v1alpha1.ProfileNames())
	}

	if strings.TrimSpace(c.WatchdogDevice) == "" {
		return errors.New("WATCHDOG_DEVICE is empty")
	}

	if c.WatchdogFeedInterval <= 0 {
		return errors.New("WATCHDOG_FEED_INTERVAL must be positive")
	}

	if c.WatchdogTimeout <= 0 {
		return errors.New("WATCHDOG_TIMEOUT must be positive")
	}

	if c.WatchdogFeedInterval >= c.WatchdogTimeout {
		return fmt.Errorf("WATCHDOG_FEED_INTERVAL=%s must be less than WATCHDOG_TIMEOUT=%s", c.WatchdogFeedInterval, c.WatchdogTimeout)
	}

	if c.FallbackHeartbeat <= 0 {
		return errors.New("FALLBACK_HEARTBEAT must be positive")
	}

	if c.FallbackTTL <= 0 {
		return errors.New("FALLBACK_TTL must be positive")
	}

	if c.FallbackHeartbeat >= c.FallbackTTL {
		return fmt.Errorf("FALLBACK_HEARTBEAT=%s must be less than FALLBACK_TTL=%s", c.FallbackHeartbeat, c.FallbackTTL)
	}

	if c.EvacuationDelay <= 0 {
		return errors.New("EVACUATION_DELAY must be positive")
	}

	if c.RejoinInterval <= 0 {
		return errors.New("REJOIN_INTERVAL must be positive")
	}

	if c.RejoinMaxInterval <= 0 {
		return errors.New("REJOIN_MAX_INTERVAL must be positive")
	}

	if c.RejoinInterval > c.RejoinMaxInterval {
		return fmt.Errorf("REJOIN_INTERVAL=%s must not exceed REJOIN_MAX_INTERVAL=%s", c.RejoinInterval, c.RejoinMaxInterval)
	}

	if c.KubernetesAPITimeout <= 0 {
		return errors.New("KUBERNETES_API_TIMEOUT must be positive")
	}

	if strings.TrimSpace(c.APISocketPath) == "" {
		return errors.New("API_SOCKET_PATH is empty")
	}

	if strings.TrimSpace(c.HealthProbeBindAddress) == "" {
		return errors.New("HEALTH_PROBE_BIND_ADDRESS is empty")
	}

	if _, err := log.ParseLevel(c.LogLevel); err != nil {
		return fmt.Errorf("LOG_LEVEL=%q is invalid, must be one of trace/debug/info/warn/error/fatal", c.LogLevel)
	}

	return nil
}
