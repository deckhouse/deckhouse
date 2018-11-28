package kube_config_manager

import (
	"fmt"
	"github.com/deckhouse/deckhouse/antiopa/utils"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
)

// GetModulesNamesFromConfigData returns all keys in kube config except global
func GetModulesNamesFromConfigData(configData map[string]string) map[string]bool {
	res := make(map[string]bool, 0)

	for key := range configData {
		if key != utils.GlobalValuesKey {
			if utils.ModuleNameToValuesKey(utils.ModuleNameFromValuesKey(key)) != key {
				rlog.Warnf("Bad module name '%s': should be camelCased module name: ignoring data", key)
				continue
			}
			res[utils.ModuleNameFromValuesKey(key)] = true
		}
	}

	return res
}

type ModuleKubeConfig struct {
	utils.ModuleConfig
	Checksum   string
	ConfigData map[string]string
}

func GetModuleKubeConfigFromValues(moduleName string, values utils.Values) *ModuleKubeConfig {
	moduleValues, hasKey := values[utils.ModuleNameToValuesKey(moduleName)]
	if !hasKey {
		return nil
	}

	yamlData, err := yaml.Marshal(&moduleValues)
	if err != nil {
		panic(fmt.Sprintf("cannot dump yaml for module '%s' kube config: %s\nfailed values data: %#v", moduleName, err, moduleValues))
	}

	return &ModuleKubeConfig{
		ModuleConfig: utils.ModuleConfig{
			ModuleName: moduleName,
			IsEnabled:  true,
			Values:     utils.Values{utils.ModuleNameToValuesKey(moduleName): moduleValues},
		},
		ConfigData: map[string]string{utils.ModuleNameToValuesKey(moduleName): string(yamlData)},
		Checksum:   utils.CalculateChecksum(string(yamlData)),
	}
}

func ModuleKubeConfigMustExist(res *ModuleKubeConfig, err error) (*ModuleKubeConfig, error) {
	if err != nil {
		return res, err
	}
	if res == nil {
		panic("module kube config must exist!")
	}
	return res, err
}

func GetModuleKubeConfigFromConfigData(moduleName string, configData map[string]string) (*ModuleKubeConfig, error) {
	yamlData, hasKey := configData[utils.ModuleNameToValuesKey(moduleName)]
	if !hasKey {
		return nil, nil
	}

	moduleConfig, err := NewModuleConfig(moduleName, yamlData)
	if err != nil {
		return nil, fmt.Errorf("'%s' ConfigMap bad yaml at key '%s': %s", ConfigMapName, utils.ModuleNameToValuesKey(moduleName), err)
	}

	return &ModuleKubeConfig{
		ModuleConfig: *moduleConfig,
		Checksum:     utils.CalculateChecksum(yamlData),
	}, nil
}

func NewModuleConfig(moduleName string, moduleYamlData string) (*utils.ModuleConfig, error) {
	var valuesAtModuleKey interface{}

	err := yaml.Unmarshal([]byte(moduleYamlData), &valuesAtModuleKey)
	if err != nil {
		return nil, err
	}

	data := map[interface{}]interface{}{utils.ModuleNameToValuesKey(moduleName): valuesAtModuleKey}

	return utils.NewModuleConfig(moduleName).WithValues(data)
}
