package kube_config_manager

import (
	"fmt"
	"time"

	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	_ "github.com/deckhouse/deckhouse/antiopa/utils"
)

const (
	GlobalValuesKeyName = "global"
)

type Config struct {
	Values        map[interface{}]interface{}
	ModuleConfigs map[string]ModuleConfig
}

type ModuleConfig struct {
	ModuleName string
	IsEnabled  bool
	Values     map[interface{}]interface{}
}

/* TODO
SetModuleKubeValues
*/

var (
	ConfigUpdated       <-chan Config
	ModuleConfigUpdated <-chan ModuleConfig
)

func Init() (*Config, error) {
	rlog.Debug("Init kube config manager")

	res := &Config{
		Values:        make(map[interface{}]interface{}),
		ModuleConfigs: make(map[string]ModuleConfig),
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
			err := yaml.Unmarshal(valuesYaml, &res.Values)
			if err != nil {
				return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s:\n%s", kube.AntiopaSecret, GlobalValuesKeyName, err, string(valuesYaml))
			}
		}

		for key, value := range secret.Data {
			if key != GlobalValuesKeyName {
				moduleConfig := ModuleConfig{
					ModuleName: key,
					IsEnabled:  true,
					Values:     make(map[interface{}]interface{}),
				}

				var valueData interface{}

				err := yaml.Unmarshal(value, &valueData)
				if err != nil {
					return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s:\n%s", kube.AntiopaSecret, moduleConfig.ModuleName, err, string(value))
				}

				if moduleEnabled, isBool := valueData.(bool); isBool {
					moduleConfig.IsEnabled = moduleEnabled
				} else {
					moduleValues, moduleValuesOk := valueData.(map[interface{}]interface{})
					if !moduleValuesOk {
						return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': expected map or bool, got: %s")
					}
					moduleConfig.Values = moduleValues
				}

				res.ModuleConfigs[moduleConfig.ModuleName] = moduleConfig
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
