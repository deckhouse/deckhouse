/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"log"

	"github.com/spf13/viper"
)

type FileConfig struct {
	HostName       string `mapstructure:"hostName"`
	MyIP           string `mapstructure:"myIP"`
	MyPodName      string `mapstructure:"myPodName"`
	LeaderElection struct {
		Namespace            string `mapstructure:"namespace"`
		LeaseDurationSeconds int    `mapstructure:"leaseDurationSeconds"`
		RenewDeadlineSeconds int    `mapstructure:"renewDeadlineSeconds"`
		RetryPeriodSeconds   int    `mapstructure:"retryPeriodSeconds"`
	}
}

func NewFileConfig() (*FileConfig, error) {
	var cfg *FileConfig
	viper.SetConfigFile(GetConfigFilePath())
	viper.SetDefault("LeaderElection.LeaseDurationSeconds", 3600)
	viper.SetDefault("LeaderElection.RenewDeadlineSeconds", 10)
	viper.SetDefault("LeaderElection.RetryPeriodSeconds", 4)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	bindEnv("HostName", "HOSTNAME")
	bindEnv("MyIP", "MY_IP")
	bindEnv("MyPodName", "MY_POD_NAME")
	bindEnv("LeaderElection.Namespace", "LEADER_ELECTION_NAMESPACE")
	bindEnv("LeaderElection.LeaseDurationSeconds", "LEADER_ELECTION_LEASE_DURATION_SECONDS")
	bindEnv("LeaderElection.RenewDeadlineSeconds", "LEADER_ELECTION_RENEW_DEADLINE_SECONDS")
	bindEnv("LeaderElection.RetryPeriodSeconds", "LEADER_ELECTION_RETRY_PERIOD_SECONDS")

	validateConfigEntry(
		"HostName",
		"HOSTNAME",
	)

	validateConfigEntry(
		"MyIP",
		"MY_IP",
	)

	validateConfigEntry(
		"MyPodName",
		"MY_POD_NAME",
	)

	validateConfigEntry(
		"LeaderElection.Namespace",
		"LEADER_ELECTION_NAMESPACE",
	)

	validateConfigEntry(
		"LeaderElection.LeaseDurationSeconds",
		"LEADER_ELECTION_LEASE_DURATION_SECONDS",
	)

	validateConfigEntry(
		"LeaderElection.RenewDeadlineSeconds",
		"LEADER_ELECTION_RENEW_DEADLINE_SECONDS",
	)

	validateConfigEntry(
		"LeaderElection.RetryPeriodSeconds",
		"LEADER_ELECTION_RETRY_PERIOD_SECONDS",
	)

	viper.AutomaticEnv()
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Error unmarshaling config: %v", err)
	}

	return cfg, nil
}

func bindEnv(configKey, envVar string) {
	if err := viper.BindEnv(configKey, envVar); err != nil {
		log.Fatalf("Error binding %s: %v", configKey, err)
	}
}

func validateConfigEntry(entry, envVar string) {
	if !viper.IsSet(entry) || viper.GetString(entry) == "" {
		log.Fatalf("%s is not set or empty. Please configure it in the configuration file or via the environment variable %s.", entry, envVar)
	}
}
