package kube_config_manager

import (
	"fmt"
	"github.com/deckhouse/deckhouse/antiopa/utils"
	"gopkg.in/yaml.v2"
)

type GlobalKubeConfig struct {
	Values     utils.Values
	Checksum   string
	ConfigData map[string]string
}

func GetGlobalKubeConfigFromValues(values utils.Values) *GlobalKubeConfig {
	globalValues, hasKey := values[utils.GlobalValuesKey]
	if !hasKey {
		return nil
	}

	yamlData, err := yaml.Marshal(&globalValues)
	if err != nil {
		panic(fmt.Sprintf("cannot dump yaml for global kube config: %s\nfailed values data: %#v", err, globalValues))
	}

	return &GlobalKubeConfig{
		Values:     utils.Values{utils.GlobalValuesKey: globalValues},
		Checksum:   utils.CalculateChecksum(string(yamlData)),
		ConfigData: map[string]string{utils.GlobalValuesKey: string(yamlData)},
	}
}

func GetGlobalKubeConfigFromConfigData(configData map[string]string) (*GlobalKubeConfig, error) {
	yamlData, hasKey := configData[utils.GlobalValuesKey]
	if !hasKey {
		return nil, nil
	}

	values, err := NewGlobalValues(yamlData)
	if err != nil {
		return nil, fmt.Errorf("'%s' ConfigMap bad yaml at key '%s': %s:\n%s", ConfigMapName, utils.GlobalValuesKey, err, string(yamlData))
	}

	return &GlobalKubeConfig{
		ConfigData: map[string]string{utils.GlobalValuesKey: yamlData},
		Values:     values,
		Checksum:   utils.CalculateChecksum(yamlData),
	}, nil
}

func NewGlobalValues(yamlData string) (utils.Values, error) {
	var dataMap map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(yamlData), &dataMap)
	if err != nil {
		return nil, err
	}
	data := map[interface{}]interface{}{utils.GlobalValuesKey: dataMap}

	values, err := utils.NewValues(data)
	if err != nil {
		return nil, err
	}

	return values, nil
}
