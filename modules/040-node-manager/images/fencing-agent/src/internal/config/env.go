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
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Env holds the values from the pod environment. The fencing configuration
// is not here: it comes from the ConfigMap.
type Env struct {
	NodeName               string `env:"NODE_NAME" env-required:"true"`
	WatchdogDevice         string `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	APISocketPath          string `env:"API_SOCKET_PATH" env-default:"/var/run/fencing-agent/fencing-agent.sock"`
	HealthProbeBindAddress string `env:"HEALTH_PROBE_BIND_ADDRESS" env-default:"127.0.0.1:4250"`
	LogLevel               string `env:"LOG_LEVEL" env-default:"info"`
	// KeySwitchDelay is the time from the addition of a new memberlist key to
	// the keyring to the switch of the primary key, and the shortest time from
	// the switch to the removal of the old key.
	KeySwitchDelay time.Duration `env:"KEY_SWITCH_DELAY" env-default:"2m"`
}

// LoadEnv reads and validates the environment.
func LoadEnv() (Env, error) {
	var e Env
	if err := cleanenv.ReadEnv(&e); err != nil {
		return Env{}, fmt.Errorf("read environment: %w", err)
	}

	if err := e.validate(); err != nil {
		return Env{}, fmt.Errorf("validate environment: %w", err)
	}

	return e, nil
}

func (e Env) validate() error {
	for _, v := range []struct{ name, value string }{
		{"NODE_NAME", e.NodeName},
		{"WATCHDOG_DEVICE", e.WatchdogDevice},
		{"API_SOCKET_PATH", e.APISocketPath},
		{"HEALTH_PROBE_BIND_ADDRESS", e.HealthProbeBindAddress},
	} {
		if strings.TrimSpace(v.value) == "" {
			return errors.New(v.name + " is empty")
		}
	}

	if _, err := log.ParseLevel(e.LogLevel); err != nil {
		return fmt.Errorf("LOG_LEVEL=%q is invalid, must be one of trace/debug/info/warn/error/fatal", e.LogLevel)
	}

	if e.KeySwitchDelay <= 0 {
		return fmt.Errorf("KEY_SWITCH_DELAY=%s is invalid, must be positive", e.KeySwitchDelay)
	}

	return nil
}
