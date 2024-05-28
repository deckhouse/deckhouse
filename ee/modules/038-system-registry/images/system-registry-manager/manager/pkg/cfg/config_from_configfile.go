/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cfg

import (
	"fmt"
	"log"

	"reflect"
	pkg_utils "system-registry-manager/pkg/utils"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type FileConfig struct {
	HostName string `mapstructure:"hostName"`
	HostIP   string `mapstructure:"hostIP"`
	PodName  string `mapstructure:"podName"`
	Manager  struct {
		Namespace      string `mapstructure:"namespace"`
		DaemonsetName  string `mapstructure:"daemonsetName"`
		ServiceName    string `mapstructure:"serviceName"`
		WorkerPort     int    `mapstructure:"workerPort"`
		LeaderElection struct {
			LeaseDurationSeconds int `mapstructure:"leaseDurationSeconds"`
			RenewDeadlineSeconds int `mapstructure:"renewDeadlineSeconds"`
			RetryPeriodSeconds   int `mapstructure:"retryPeriodSeconds"`
		} `mapstructure:"leaderElection"`
	} `mapstructure:"manager"`
	Etcd struct {
		Addresses []string `mapstructure:"addresses"`
	} `mapstructure:"etcd"`
	Registry struct {
		RegistryMode     string `mapstructure:"registryMode"`
		UpstreamRegistry struct {
			UpstreamRegistryHost     string `mapstructure:"upstreamRegistryHost"`
			UpstreamRegistryScheme   string `mapstructure:"upstreamRegistryScheme"`
			UpstreamRegistryCa       string `mapstructure:"upstreamRegistryCa"`
			UpstreamRegistryPath     string `mapstructure:"upstreamRegistryPath"`
			UpstreamRegistryUser     string `mapstructure:"upstreamRegistryUser"`
			UpstreamRegistryPassword string `mapstructure:"upstreamRegistryPassword"`
		} `mapstructure:"upstreamRegistry"`
	} `mapstructure:"registry"`
	Images struct {
		SystemRegistry struct {
			DockerDistribution string `mapstructure:"dockerDistribution"`
			DockerAuth         string `mapstructure:"dockerAuth"`
			Seaweedfs          string `mapstructure:"seaweedfs"`
		} `mapstructure:"systemRegistry"`
	} `mapstructure:"images"`
}

func GetDefaultConfigVars() []ConfigVar {
	defaultConfigVars := []ConfigVar{
		{Key: "hostName", Env: CreateEnv("HOSTNAME"), Default: nil},
		{Key: "hostIP", Env: CreateEnv("HOST_IP"), Default: nil},
		{Key: "podName", Env: CreateEnv("POD_NAME"), Default: nil},
		{Key: "manager.workerPort", Env: nil, Default: CreateDefaultValue(8097)},
		{Key: "manager.leaderElection.leaseDurationSeconds", Env: nil, Default: CreateDefaultValue(15)},
		{Key: "manager.leaderElection.renewDeadlineSeconds", Env: nil, Default: CreateDefaultValue(10)},
		{Key: "manager.leaderElection.retryPeriodSeconds", Env: nil, Default: CreateDefaultValue(2)},
		{Key: "registry.upstreamRegistry.upstreamRegistryCa", Env: nil, Default: CreateDefaultValue("")},
	}
	{
		defaultKeys := make([]string, 0, len(defaultConfigVars))
		for _, defaultConfigVar := range defaultConfigVars {
			defaultKeys = append(defaultKeys, defaultConfigVar.Key)
		}

		extraConfigVars := []ConfigVar{}
		for _, key := range GetAllMapstructureKeys(FileConfig{}) {
			if !pkg_utils.IsStringInSlice(key, &defaultKeys) {
				extraConfigVars = append(
					extraConfigVars,
					ConfigVar{Key: key, Env: nil, Default: nil},
				)
			}
		}
		defaultConfigVars = append(defaultConfigVars, extraConfigVars...)
	}
	return defaultConfigVars
}

func (fcfg *FileConfig) DecodeToMapstructure() (map[string]interface{}, error) {
	var configMap map[string]interface{}

	err := mapstructure.Decode(fcfg, &configMap)
	if err != nil {
		return nil, fmt.Errorf("error decoding config: %v", err)
	}
	return configMap, nil
}

func NewFileConfig() (*FileConfig, error) {
	configVars := GetDefaultConfigVars()

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
		value := viper.Get(configVar.Key)
		if isZero(value) {
			if configVar.Env == nil {
				log.Fatalf("%s is not set or empty. Please configure it in the configuration file.", configVar.Key)
			} else {
				log.Fatalf("%s is not set or empty. Please configure it in the configuration file or via the environment variable %s.", configVar.Key, *configVar.Env)
			}
		}
	}
}

func isZero(value interface{}) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}

func GetAllMapstructureKeys(config interface{}) []string {
	return getKeysFromStruct(reflect.TypeOf(config), "")
}

func getKeysFromStruct(t reflect.Type, prefix string) []string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			tag = field.Name
		}
		if prefix != "" {
			tag = prefix + "." + tag
		}
		switch field.Type.Kind() {
		case reflect.Struct:
			keys = append(keys, getKeysFromStruct(field.Type, tag)...)
		case reflect.Slice, reflect.Array, reflect.Ptr:
			if field.Type.Elem().Kind() == reflect.Struct {
				keys = append(keys, getKeysFromStruct(field.Type.Elem(), tag)...)
			} else {
				keys = append(keys, tag)
			}
		default:
			keys = append(keys, tag)
		}
	}
	return keys
}
