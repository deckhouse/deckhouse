/*
Copyright 2024 Flant JSC

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

package agent

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	WatchdogDevice             string        `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	WatchdogFeedInterval       time.Duration `env:"WATCHDOG_FEED_INTERVAL" env-default:"5s"`
	KubernetesAPICheckInterval time.Duration `env:"KUBERNETES_API_CHECK_INTERVAL" env-default:"5s"`
	KubernetesAPITimeout       time.Duration `env:"KUBERNETES_API_TIMEOUT" env-default:"10s"`
	APIIsAvailableMsgInterval  time.Duration `env:"API_IS_AVAILABLE_MSG_INTERVAL" env-default:"90s"`
	HealthProbeBindAddress     string        `env:"HEALTH_PROBE_BIND_ADDRESS"  env-default:":8081"`
	NodeName                   string        `env:"NODE_NAME"`
}

func (c *Config) Load() error {
	err := cleanenv.ReadEnv(c)
	if err != nil {
		return err
	}
	return nil
}
