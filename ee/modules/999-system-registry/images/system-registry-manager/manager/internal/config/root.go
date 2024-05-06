/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

const (
	AnnotationFromMe      = `system-registry-manager.deckhouse.io/manager`
	AnnotationFromHandler = `system-registry-manager.deckhouse.io/handler`
	MaxRetries            = 120
	defaultConfigFilePath = "config.yaml"
)

var (
	config         *Config
	configFilePath string = ""
)

type Config struct {
	SystemRegistry        SystemRegistryConfig
	SystemRegistryManager SystemRegistryManagerConfig
}

func InitConfig() (*Config, error) {
	systemRegistryConfig, err := NewSystemRegistryConfig()
	if err != nil {
		return nil, err
	}
	systemRegistryManagerConfig, err := NewSystemRegistryManagerConfig()
	if err != nil {
		return nil, err
	}

	newConfig := Config{
		SystemRegistry:        *systemRegistryConfig,
		SystemRegistryManager: *systemRegistryManagerConfig,
	}
	return &newConfig, nil
}

func GetConfig() *Config {
	return config
}

func GetConfigFilePath() string {
	if configFilePath != "" {
		return configFilePath
	}
	return defaultConfigFilePath
}

func SetConfigFilePath(newConfigFilePath string) {
	configFilePath = newConfigFilePath
}
