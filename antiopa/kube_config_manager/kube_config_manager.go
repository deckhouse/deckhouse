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
	SecretName          = "antiopa"
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

func simpleMergeConfigData(data map[string][]byte, newData map[string][]byte) map[string][]byte {
	for k, v := range newData {
		data[k] = v
	}
	return data
}

func (kcm *MainKubeConfigManager) setConfigSecretData(mergeData map[string][]byte) (*v1.Secret, error) {
	secretsList, err := kube.KubernetesClient.CoreV1().
		Secrets(kube.KubernetesAntiopaNamespace).
		List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	secretExist := false
	for _, cm := range secretsList.Items {
		if cm.ObjectMeta.Name == SecretName {
			secretExist = true
			break
		}
	}

	if secretExist {
		secret, err := kube.KubernetesClient.CoreV1().
			Secrets(kube.KubernetesAntiopaNamespace).
			Get(SecretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data = simpleMergeConfigData(secret.Data, mergeData)

		return secret, nil
	} else {
		secret := &v1.Secret{}
		secret.Name = SecretName
		secret.Data = simpleMergeConfigData(make(map[string][]byte), mergeData)

		_, err := kube.KubernetesClient.CoreV1().Secrets(kube.KubernetesAntiopaNamespace).Create(secret)
		if err != nil {
			return nil, err
		}

		return secret, nil
	}
}

func (kcm *MainKubeConfigManager) SetKubeValues(values utils.Values) error {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return err
	}

	// TODO: store checksum in Secret
	_, err = kcm.setConfigSecretData(map[string][]byte{GlobalValuesKeyName: valuesYaml})
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

	// TODO: store checksum in Secret
	// FIXME: camelcase module name
	_, err = kcm.setConfigSecretData(map[string][]byte{moduleName: valuesYaml})
	if err != nil {
		return err
	}
	// TODO: store known resource-version

	return nil
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

	secretsList, err := kube.KubernetesClient.CoreV1().
		Secrets(kube.KubernetesAntiopaNamespace).
		List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	secretExist := false
	for _, secret := range secretsList.Items {
		if secret.ObjectMeta.Name == SecretName {
			secretExist = true
			break
		}
	}

	if secretExist {
		secret, err := kube.KubernetesClient.CoreV1().
			Secrets(kube.KubernetesAntiopaNamespace).
			Get(SecretName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Cannot get Secret %s from namespace %s: %s", SecretName, kube.KubernetesAntiopaNamespace, err)
		}

		if valuesYaml, hasKey := secret.Data[GlobalValuesKeyName]; hasKey {
			var values map[interface{}]interface{}
			err := yaml.Unmarshal(valuesYaml, &values)
			if err != nil {
				return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s:\n%s", SecretName, GlobalValuesKeyName, err, string(valuesYaml))
			}
			formattedValues, err := utils.FormatValues(values)
			if err != nil {
				return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s\n%s", SecretName, GlobalValuesKeyName, err, string(valuesYaml))
			}
			kcm.initialConfig.Values = formattedValues
		}

		for key, value := range secret.Data {
			if key != GlobalValuesKeyName {
				moduleConfig, err := utils.NewModuleConfigByYamlData(key, value)
				if err != nil {
					return nil, fmt.Errorf("'%s' Secret bad yaml at key '%s': %s", SecretName, key, err)
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
