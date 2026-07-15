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

	"github.com/ilyakaznacheev/cleanenv"

	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/watchdog"
	"fencing-agent/internal/controllers/grpc"
	"fencing-agent/internal/domain"
)

type Config struct {
	FencingMode            string `env:"FENCING_MODE" env-required:"true"`
	HealthProbeBindAddress string `env:"HEALTH_PROBE_BIND_ADDRESS"  env-default:":8081"`
	NodeName               string `env:"NODE_NAME" env-required:"true"`
	NodeGroup              string `env:"NODE_GROUP" env-required:"true"`
	ProfileRefName         string `env:"PROFILE_REF_NAME" env-required:"true"`
	APISocketPath          string `env:"API_SOCKET_PATH" env-default:"/var/run/fencing-agent.sock"`
	LogLevel               string `env:"LOG_LEVEL" env-default:"info"`

	Watchdog watchdog.Config

	KubeClient kubeclient.Config

	Memberlist memberlist.Config

	GRPC grpc.Config
}

func (c *Config) MustLoad() {
	readErr := cleanenv.ReadEnv(c)
	if readErr != nil {
		panic(readErr)
	}

	valErr := c.validate()
	if valErr != nil {
		panic(valErr)
	}
}

func (c *Config) validate() error {
	validators := []func() error{
		c.validateCommon,
		c.KubeClient.Validate,
		c.Watchdog.Validate,
		c.Memberlist.Validate,
		c.GRPC.Validate,
	}

	for _, validator := range validators {
		if err := validator(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) validateCommon() error {
	if strings.TrimSpace(c.NodeName) == "" {
		return errors.New("NODE_NAME is empty")
	}

	if !domain.FencingMode(c.FencingMode).IsValid() {
		return fmt.Errorf("FENCING_MODE=%q is invalid, must be one of %v", c.FencingMode, domain.FencingModes())
	}

	if strings.TrimSpace(c.NodeGroup) == "" {
		return errors.New("NODE_GROUP is empty")
	}

	if !domain.ProfileName(c.ProfileRefName).IsValid() {
		return fmt.Errorf("PROFILE_REF_NAME=%q is invalid, must be one of %v", c.ProfileRefName, domain.ProfileNames())
	}

	if strings.TrimSpace(c.LogLevel) == "" {
		return errors.New("LOG_LEVEL is empty")
	}

	if strings.TrimSpace(c.HealthProbeBindAddress) == "" {
		return errors.New("HEALTH_PROBE_BIND_ADDRESS is empty")
	}
	return nil
}
