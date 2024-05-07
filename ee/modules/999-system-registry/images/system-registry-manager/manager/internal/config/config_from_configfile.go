/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type FileConfig struct {
	NodeName  string `mapstructure:"nodeName"`
	MyIP      string `mapstructure:"myIP"`
	MyPodName string `mapstructure:"myPodName"`
	HostName  string `mapstructure:"hostName"`
}

func NewFileConfig() (*FileConfig, error) {
	var cfg *FileConfig
	viper.SetConfigFile(GetConfigFilePath())

	if err := viper.ReadInConfig(); err != nil {
		log.WithError(err).Fatal("Error reading config file")
	}

	bindEnv("NodeName", "NODE_NAME")
	bindEnv("MyIP", "MY_IP")
	bindEnv("MyPodName", "MY_POD_NAME")
	bindEnv("HostName", "HOSTNAME")

	validateConfigEntry(
		"NodeName",
		"NodeName",
		"NodeName",
		"NODE_NAME",
	)

	validateConfigEntry(
		"MyIP",
		"MyIP",
		"MyIP",
		"MY_IP",
	)

	validateConfigEntry(
		"MyPodName",
		"MyPodName",
		"MyPodName",
		"MY_POD_NAME",
	)

	validateConfigEntry(
		"HostName",
		"HostName",
		"HostName",
		"HOSTNAME",
	)

	viper.AutomaticEnv()
	if err := viper.Unmarshal(&cfg); err != nil {
		log.WithError(err).Fatal("Error unmarshaling config")
	}

	return cfg, nil
}

func bindEnv(configKey, envVar string) {
	if err := viper.BindEnv(configKey, envVar); err != nil {
		log.WithError(err).Fatalf("Error binding %s", configKey)
	}
}

func validateConfigEntry(entry, prettyName, configPath, envVar string) {
	if !viper.IsSet(entry) {
		log.Fatalf(
			"%s is not set. Please configure it in the configuration file ('%s') or via the '%s' environment variable.",
			prettyName,
			configPath,
			envVar,
		)
	} else if viper.GetString(entry) == "" {
		log.Fatalf("%s is empty. Please provide a valid value in the '%s' file ('%s') or via the '%s' environment variable.", GetConfigFilePath(), prettyName, configPath, envVar)
	}
}
