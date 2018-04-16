package kube_config_manager

import (
	"fmt"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

const (
	GlobalValuesKeyName = "global"
	ConfigMapName       = "antiopa"
)

type KubeConfigManager interface {
	SetKubeValues(values utils.Values) error
	SetModuleKubeValues(moduleName string, values utils.Values) error
	Run()
	InitialConfig() *Config
}

type MainKubeConfigManager struct {
	initialConfig *Config
}

type Config struct {
	Values        utils.Values
	ModuleConfigs map[string]utils.ModuleConfig
}

var (
	ConfigUpdated       <-chan Config
	ModuleConfigUpdated <-chan utils.ModuleConfig
)

func simpleMergeConfigMapData(data map[string]string, newData map[string]string) map[string]string {
	for k, v := range newData {
		data[k] = v
	}
	return data
}

func (kcm *MainKubeConfigManager) setConfigData(mergeData map[string]string) (*v1.ConfigMap, error) {
	obj, err := kcm.getConfigMap()
	if err != nil {
		return nil, err
	}

	if obj != nil {
		if obj.Data == nil {
			obj.Data = make(map[string]string)
		}
		obj.Data = simpleMergeConfigMapData(obj.Data, mergeData)

		updatedObj, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).Update(obj)
		if err != nil {
			return nil, err
		}

		return updatedObj, nil
	} else {
		obj := &v1.ConfigMap{}
		obj.Name = ConfigMapName
		obj.Data = simpleMergeConfigMapData(make(map[string]string), mergeData)

		_, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).Create(obj)
		if err != nil {
			return nil, err
		}

		return obj, nil
	}
}

func (kcm *MainKubeConfigManager) SetKubeValues(values utils.Values) error {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return err
	}

	// TODO: store checksum
	_, err = kcm.setConfigData(map[string]string{GlobalValuesKeyName: string(valuesYaml)})
	if err != nil {
		return err
	}
	// TODO: store known resource-version

	return nil
}

func (kcm *MainKubeConfigManager) SetModuleKubeValues(moduleName string, values utils.Values) error {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return err
	}

	// TODO: store checksum
	_, err = kcm.setConfigData(map[string]string{utils.ModuleNameToValuesKey(moduleName): string(valuesYaml)})
	if err != nil {
		return err
	}
	// TODO: store known resource-version

	return nil
}

func (kcm *MainKubeConfigManager) getConfigMap() (*v1.ConfigMap, error) {
	list, err := kube.KubernetesClient.CoreV1().
		ConfigMaps(kube.KubernetesAntiopaNamespace).
		List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objExists := false
	for _, obj := range list.Items {
		if obj.ObjectMeta.Name == ConfigMapName {
			objExists = true
			break
		}
	}

	if objExists {
		obj, err := kube.KubernetesClient.CoreV1().
			ConfigMaps(kube.KubernetesAntiopaNamespace).
			Get(ConfigMapName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return obj, nil
	} else {
		return nil, nil
	}
}

func (kcm *MainKubeConfigManager) InitialConfig() *Config {
	return kcm.initialConfig
}

func Init() (KubeConfigManager, error) {
	rlog.Debug("Init kube config manager")

	kcm := &MainKubeConfigManager{}
	kcm.initialConfig = &Config{
		Values:        make(utils.Values),
		ModuleConfigs: make(map[string]utils.ModuleConfig),
	}

	obj, err := kcm.getConfigMap()
	if err != nil {
		return nil, err
	}

	if obj != nil {
		if valuesYaml, hasKey := obj.Data[GlobalValuesKeyName]; hasKey {
			var values map[interface{}]interface{}
			err := yaml.Unmarshal([]byte(valuesYaml), &values)
			if err != nil {
				return nil, fmt.Errorf("'%s' ConfigMap bad yaml at key '%s': %s:\n%s", ConfigMapName, GlobalValuesKeyName, err, string(valuesYaml))
			}
			formattedValues, err := utils.FormatValues(values)
			if err != nil {
				return nil, fmt.Errorf("'%s' ConfigMap bad yaml at key '%s': %s\n%s", ConfigMapName, GlobalValuesKeyName, err, string(valuesYaml))
			}
			kcm.initialConfig.Values = formattedValues
		}

		for key, value := range obj.Data {
			if key != GlobalValuesKeyName {
				moduleConfig, err := utils.NewModuleConfigByYamlData(utils.ModuleNameFromValuesKey(key), []byte(value))
				if err != nil {
					return nil, fmt.Errorf("'%s' ConfigMap bad yaml at key '%s': %s", ConfigMapName, key, err)
				}
				kcm.initialConfig.ModuleConfigs[moduleConfig.ModuleName] = *moduleConfig
			}
		}
	}

	return kcm, nil
}

func (kcm *MainKubeConfigManager) Run() {
	rlog.Debugf("Run kube config manager")

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}
