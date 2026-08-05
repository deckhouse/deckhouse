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

	"github.com/ilyakaznacheev/cleanenv"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

type Config struct {
	NodeName       string `env:"NODE_NAME" env-required:"true"`
	NodeUID        string
	NodeGroup      string `env:"NODE_GROUP" env-required:"true"`
	ProfileRefName string `env:"PROFILE_REF_NAME" env-required:"true"`

	MemberlistPort int `env:"MEMBERLIST_PORT" env-default:"8500"`

	WatchdogDevice string `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`

	APISocketPath          string `env:"API_SOCKET_PATH" env-default:"/var/run/fencing-agent/fencing-agent.sock"`
	HealthProbeBindAddress string `env:"HEALTH_PROBE_BIND_ADDRESS" env-default:":8081"`
	LogLevel               string `env:"LOG_LEVEL" env-default:"info"`
}

// Load reads and validates the environment. Fencing timings deliberately have
// no environment variables: they come only from the FencingSLAProfile named by
// PROFILE_REF_NAME, and without a valid profile the agent must not run.
func (c *Config) Load() error {
	if err := cleanenv.ReadEnv(c); err != nil {
		return fmt.Errorf("read environment: %w", err)
	}

	if err := c.validate(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}

	return nil
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

	if c.MemberlistPort < 1 || c.MemberlistPort > 65535 {
		return fmt.Errorf("MEMBERLIST_PORT=%d is out of range 1-65535", c.MemberlistPort)
	}

	if strings.TrimSpace(c.WatchdogDevice) == "" {
		return errors.New("WATCHDOG_DEVICE is empty")
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
