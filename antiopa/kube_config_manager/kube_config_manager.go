package kube_config_manager

import (
	"fmt"
	"time"

	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

const (
	GlobalValuesKeyName = "global"
)

type Config struct {
	Values        utils.Values
	ModuleConfigs map[string]utils.ModuleConfig
}

/* TODO
SetModuleKubeValues
*/

var (
	ConfigUpdated       <-chan Config
	ModuleConfigUpdated <-chan utils.ModuleConfig
)

func Init() (*Config, error) {
	rlog.Debug("Init kube config manager")

	res := &Config{
		Values:        make(utils.Values),
		ModuleConfigs: make(map[string]utils.ModuleConfig),
	}

	secretsList, err := kube.KubernetesClient.CoreV1().Secrets(kube.KubernetesAntiopaNamespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	secretExist := false
	for _, secret := range secretsList.Items {
		if secret.ObjectMeta.Name == kube.AntiopaSecret {
			secretExist = true
			break
		}
	}

	if secretExist {
		secret, err := kube.KubernetesClient.CoreV1().
			Secrets(kube.KubernetesAntiopaNamespace).
			Get(kube.AntiopaSecret, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Cannot get Secret %s from namespace %s: %s", kube.AntiopaSecret, kube.KubernetesAntiopaNamespace, err)
		}

		if valuesYaml, hasKey := secret.Data[GlobalValuesKeyName]; hasKey {
			var values map[interface{}]interface{}
			err := yaml.Unmarshal(valuesYaml, &values)
			if err != nil {
				return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s:\n%s", kube.AntiopaSecret, GlobalValuesKeyName, err, string(valuesYaml))
			}
			formattedValues, err := utils.FormatValues(values)
			if err != nil {
				return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s\n%s", kube.AntiopaSecret, GlobalValuesKeyName, err, string(valuesYaml))
			}
			res.Values = formattedValues
		}

		for key, value := range secret.Data {
			if key != GlobalValuesKeyName {
				var valueData interface{}
				err := yaml.Unmarshal(value, &valueData)
				if err != nil {
					return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s:\n%s", kube.AntiopaSecret, key, err, string(value))
				}

				moduleConfig, err := utils.NewModuleConfig(key, valueData)
				if err != nil {
					return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s", kube.AntiopaSecret, key, err)
				}
				res.ModuleConfigs[moduleConfig.ModuleName] = *moduleConfig
			}
		}
	}

	return res, nil
}

func RunManager() {
	rlog.Debugf("Run kube config manager")

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}
