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
	HostIP         string `mapstructure:"hostIP"`
	PodName        string `mapstructure:"podName"`
	LeaderElection struct {
		Namespace            string `mapstructure:"namespace"`
		LeaseDurationSeconds int    `mapstructure:"leaseDurationSeconds"`
		RenewDeadlineSeconds int    `mapstructure:"renewDeadlineSeconds"`
		RetryPeriodSeconds   int    `mapstructure:"retryPeriodSeconds"`
	} `mapstructure:"leaderElection"`
	Etcd struct {
		Addresses []string `mapstructure:"addresses"`
	} `mapstructure:"etcd"`
	Distribution struct {
		Image string `mapstructure:"image"`
	} `mapstructure:"distribution"`
	Auth struct {
		Image string `mapstructure:"image"`
	} `mapstructure:"auth"`
	Seaweedfs struct {
		Image string `mapstructure:"image"`
	} `mapstructure:"seaweedfs"`
}

func NewFileConfig() (*FileConfig, error) {
	configVars := []ConfigVar{
		{Key: "HostName", Env: CreateEnv("HOSTNAME"), Default: nil},
		{Key: "HostIP", Env: CreateEnv("HOST_IP"), Default: nil},
		{Key: "PodName", Env: CreateEnv("POD_NAME"), Default: nil},
		{Key: "LeaderElection.LeaseDurationSeconds", Env: nil, Default: CreateDefaultValue(15)},
		{Key: "LeaderElection.RenewDeadlineSeconds", Env: nil, Default: CreateDefaultValue(10)},
		{Key: "LeaderElection.RetryPeriodSeconds", Env: nil, Default: CreateDefaultValue(2)},
	}

	var cfg FileConfig
	viper.SetConfigFile(GetConfigFilePath())
	viper.SetConfigType("yaml")

	for _, configVar := range configVars {
		setDefault(&configVar)
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	for _, configVar := range configVars {
		bindEnvAndValidate(&configVar)
	}

	viper.AutomaticEnv()
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Error unmarshaling config: %v", err)
	}
	return &cfg, nil
}

type ConfigVar struct {
	Key     string
	Env     *string
	Default *interface{}
}

func CreateDefaultValue(defaultValue interface{}) *interface{} {
	return &defaultValue
}

func CreateEnv(env string) *string {
	return &env
}

func setDefault(configVar *ConfigVar) {
	if configVar.Default != nil {
		viper.SetDefault(configVar.Key, *configVar.Default)
	}
}

func bindEnvAndValidate(configVar *ConfigVar) {
	if configVar.Env != nil {
		if err := viper.BindEnv(configVar.Key, *configVar.Env); err != nil {
			log.Fatalf("Error binding %s: %v", configVar.Key, err)
		}
	}
	if configVar.Default == nil {
		if !viper.IsSet(configVar.Key) || viper.GetString(configVar.Key) == "" {
			if configVar.Env == nil {
				log.Fatalf("%s is not set or empty. Please configure it in the configuration file.", configVar.Key)
			} else {
				log.Fatalf("%s is not set or empty. Please configure it in the configuration file or via the environment variable %s.", configVar.Key, *configVar.Env)
			}
		}
	}
}
