package fencingconfig

import (
	"errors"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Watchdog               WatchdogConfig
	Memberlist             MemberlistConfig
	GRPC                   GRPCConfig
	KubeAPI                KubeConfig
	HealthProbeBindAddress string `env:"HEALTH_PROBE_BIND_ADDRESS"  env-default:":8081"`
	NodeName               string `env:"NODE_NAME" env-required:"true"`
	NodeGroup              string `env:"NODE_GROUP" env-required:"true"`
	LogLevel               string `env:"LOG_LEVEL" env-default:"info"`
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
		c.GRPC.validate,
		c.Memberlist.validate,
		c.Watchdog.validate,
		c.KubeAPI.validate,
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

	if strings.TrimSpace(c.NodeGroup) == "" {
		return errors.New("NODE_GROUP is empty")
	}

	if strings.TrimSpace(c.LogLevel) == "" {
		return errors.New("LOG_LEVEL is empty")
	}

	if strings.TrimSpace(c.HealthProbeBindAddress) == "" {
		return errors.New("HEALTH_PROBE_BIND_ADDRESS is empty")
	}
	return nil
}
