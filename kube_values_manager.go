package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	_ "time"

	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/* Формат values:
data:
	values: |
		<values-yaml>
	<module-name>-values: |
		<values-yaml>
	...
  	<module-name>-values: |
		<values-yaml>
	<module-name>-checksum: <checksum-of-values-yaml> // устанавливается самой antiopa
*/

const AntiopaConfigMap = "antiopa"

var (
	KubeValuesUpdated       chan map[interface{}]interface{}
	KubeModuleValuesUpdated chan KubeModuleValuesUpdate

	kubeValuesChecksum         string
	kubeModulesValuesChecksums map[string]string
	knownCmResourceVersion     string
)

type KubeModuleValuesUpdate struct {
	ModuleName string
	Values     map[interface{}]interface{}
}

func getConfigMap() (*v1.ConfigMap, error) {
	configMap, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).Get(AntiopaConfigMap, meta_v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Cannot get ConfigMap %s from namespace %s: %s", AntiopaConfigMap, KubernetesAntiopaNamespace, err)
	}

	// Data может придти пустой - нужно создать карту перед добавлением туда значений.
	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	return configMap, nil
}

func SetModuleKubeValues(ModuleName string, Values map[interface{}]interface{}) error {
	/*
	* Читаем текущий ConfigMap, создать если нету
	* Обновляем <module-name>-values + <module-name>-checksum (md5 от yaml-values)
	 */

	cmList, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).List(meta_v1.ListOptions{})
	if err != nil {
		return err
	}
	cmExist := false
	for _, cm := range cmList.Items {
		if cm.ObjectMeta.Name == "antiopa" {
			cmExist = true
			break
		}
	}

	valuesYaml, err := yaml.Marshal(&Values)
	if err != nil {
		return err
	}
	checksum := calculateChecksum(string(valuesYaml))

	if cmExist {
		cm, err := getConfigMap()
		if err != nil {
			return err
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		cm.Data[fmt.Sprintf("%s-values", ModuleName)] = string(valuesYaml)
		cm.Data[fmt.Sprintf("%s-checksum", ModuleName)] = checksum

		updatedCm, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).Update(cm)
		if err != nil {
			return err
		}
		knownCmResourceVersion = updatedCm.ResourceVersion
	} else {
		cm := v1.ConfigMap{}
		cm.Name = AntiopaConfigMap
		cm.Data = make(map[string]string)

		cm.Data[fmt.Sprintf("%s-values", ModuleName)] = string(valuesYaml)
		cm.Data[fmt.Sprintf("%s-checksum", ModuleName)] = checksum

		updatedCm, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).Create(&cm)
		if err != nil {
			return err
		}
		knownCmResourceVersion = updatedCm.ResourceVersion
	}

	return nil
}

type KubeValues struct {
	Values        map[interface{}]interface{}
	ModulesValues map[string]map[interface{}]interface{}
}

func getConfigMapValues(CM *v1.ConfigMap) (map[interface{}]interface{}, error) {
	var res map[interface{}]interface{}

	if valuesYamlStr, hasKey := CM.Data["values"]; hasKey {
		err := yaml.Unmarshal([]byte(valuesYamlStr), &res)
		if err != nil {
			return nil, fmt.Errorf("Bad ConfigMap yaml at key 'values': %s", err)
		}
	}

	return res, nil
}

func getConfigMapModulesValues(CM *v1.ConfigMap) (map[string]map[interface{}]interface{}, error) {
	res := make(map[string]map[interface{}]interface{})

	for key, value := range CM.Data {
		if strings.HasSuffix(key, "-values") {
			moduleName := strings.TrimSuffix(key, "-values")

			var moduleValues map[interface{}]interface{}

			err := yaml.Unmarshal([]byte(value), &moduleValues)
			if err != nil {
				return nil, fmt.Errorf("Bad ConfigMap yaml at key '%s': %s", key, err)
			}

			res[moduleName] = moduleValues
		}
	}

	return res, nil
}

func calculateChecksum(Data string) string {
	hasher := md5.New()
	hasher.Write([]byte(Data))
	return hex.EncodeToString(hasher.Sum(nil))
}

func InitKubeValuesManager() (KubeValues, error) {
	rlog.Debug("Init kube values manager")

	kubeModulesValuesChecksums = make(map[string]string)

	var res KubeValues
	res.Values = make(map[interface{}]interface{})
	res.ModulesValues = make(map[string]map[interface{}]interface{})

	cmList, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).List(meta_v1.ListOptions{})
	if err != nil {
		return KubeValues{}, err
	}
	cmExist := false
	for _, cm := range cmList.Items {
		if cm.ObjectMeta.Name == AntiopaConfigMap {
			cmExist = true
			break
		}
	}

	if cmExist {
		cm, err := getConfigMap()
		if err != nil {
			return KubeValues{}, err
		}

		if valuesYaml, hasKey := cm.Data["values"]; hasKey {
			err := yaml.Unmarshal([]byte(valuesYaml), &res.Values)
			if err != nil {
				return KubeValues{}, fmt.Errorf("Bad ConfigMap yaml at key 'values': %s", err)
			}
			kubeValuesChecksum = calculateChecksum(valuesYaml)
		}

		for key, value := range cm.Data {
			if strings.HasSuffix(key, "-values") {
				moduleName := strings.TrimSuffix(key, "-values")

				moduleValues := make(map[interface{}]interface{})

				err := yaml.Unmarshal([]byte(value), &moduleValues)
				if err != nil {
					return KubeValues{}, fmt.Errorf("Bad ConfigMap yaml at key '%s': %s", key, err)
				}

				res.ModulesValues[moduleName] = moduleValues
				kubeModulesValuesChecksums[moduleName] = calculateChecksum(value)
			}
		}

		modulesValues, err := getConfigMapModulesValues(cm)
		if err != nil {
			return KubeValues{}, err
		}
		for moduleName := range modulesValues {
			kubeModulesValuesChecksums[moduleName] = calculateChecksum(cm.Data[fmt.Sprintf("%s-values", moduleName)])
		}

		knownCmResourceVersion = cm.ResourceVersion
	}

	return res, nil
}

func RunKubeValuesManager() {
	rlog.Debug("Run kube values manager")

	/*
		Это горутина, поэтому в цикле.
		Long-polling через kubernetes-api через watch-запрос (* https://v1-6.docs.kubernetes.io/docs/api-reference/v1.6/#watch-199)
		* Делаем watch-запрос на ресурс ConfigMap, указывая в параметре resourceVersion известную нам версию из глобальной переменной
		* Указыаем в watch-запрос timeout в 15сек. Т.к. любой http-запрос имеет timeout, то это просто означает что надо повторить запрос по http.
		* Если resource-version поменялся, то kubernetes возвращает какой-то ответ с новым ресурсом
			* запоминаем в глобальную переменную новый resourceVersion
			* читаем values из этого ConfigMap
				* Считаем md5 от yaml-строки, если поменялась, то обновляем глобальную переменную и генерим сигнал в KubeValuesUpdated
			* читаем все module values из этого же ConfigMap, для каждого
				* Считаем md5 от yaml-строки -> фактический хэш
				* Если фактический хэш совпадает с <module-name>-checksum => не делаем ничего
				* Если фактический хэш не совпадает с <module-name>-checksum
					* Если фактический хэш не совпадает с moduleValuesChecksum[module-name]
						* Обновляем moduleValuesChecksum[module-name], генерим сигнал KubeModuleValuesUpdated
				* Считаем md5 от yaml-строки, если поменялась, то обновляем глобальную переменную и генерим сигнал в ModuleValuesUpdate
	*/
}
