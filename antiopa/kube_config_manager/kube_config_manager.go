package kube_config_manager

import (
	"time"

	"github.com/romana/rlog"
)

type Config struct {
	Values        map[string]interface{}
	ModuleConfigs map[string]ModuleConfig
}

type ModuleConfig struct {
	ModuleName string
	Values     map[string]interface{}
}

var (
	ConfigUpdated       <-chan Config
	ModuleConfigUpdated <-chan ModuleConfig
)

func Init() (*Config, error) {
	/*
	 * TODO: Init manager and return current kube-config from Secret "antiopa"
	 */

	return &Config{
		Values:        make(map[string]interface{}),
		ModuleConfigs: make(map[string]ModuleConfig),
	}, nil
}

func RunManager() {
	rlog.Debugf("Run kube config manager")

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}
