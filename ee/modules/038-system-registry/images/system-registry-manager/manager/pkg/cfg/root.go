/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cfg

import (
	"time"
)

const (
	AnnotationFromMe      = `system-registry-manager.deckhouse.io/manager`
	AnnotationFromHandler = `system-registry-manager.deckhouse.io/handler`
	MaxRetries            = 120
	CertExparationTime    = 30 * 24 * time.Hour
)

var (
	config         *Config
	configFilePath string = "./config.yaml"
)

type Config struct {
	FileConfig
	RuntimeConfig
}

func InitConfig() error {
	fileConfig, err := NewFileConfig()
	if err != nil {
		return err
	}

	runtimeConfig, err := NewRuntimeConfig()
	if err != nil {
		return err
	}

	config = &Config{
		*fileConfig,
		*runtimeConfig,
	}
	return nil
}

func InitConfigForTests(fileConfig FileConfig) error {
	config = &Config{
		fileConfig,
		RuntimeConfig{},
	}
	return nil
}

func GetConfig() *Config {
	return config
}

func GetConfigFilePath() string {
	return configFilePath
}

func SetConfigFilePath(newConfigFilePath string) {
	configFilePath = newConfigFilePath
}
