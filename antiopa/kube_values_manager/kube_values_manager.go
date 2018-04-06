package kube_values_manager

import (
	"fmt"
	"strings"
	"time"

	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/utils"
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

var (
	KubeValuesUpdated       chan KubeValues
	KubeModuleValuesUpdated chan KubeModuleValuesUpdate

	kubeValuesChecksum         string
	kubeModulesValuesChecksums map[string]string
	knownCmResourceVersion     string
)

type KubeModuleValuesUpdate struct {
	ModuleName string
	Values     map[interface{}]interface{}
}

type KubeValues struct {
	Values        map[interface{}]interface{}
	ModulesValues map[string]map[interface{}]interface{}
}

func SetKubeValues(_ utils.Values) error {
	return nil
}

func SetModuleKubeValues(ModuleName string, Values utils.Values) error {
	/*
	* Читаем текущий ConfigMap, создать если нету
	* Обновляем <module-name>-values + <module-name>-checksum (md5 от yaml-values)
	 */

	cmList, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).List(metav1.ListOptions{})
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
	checksum := utils.CalculateChecksum(string(valuesYaml))

	if cmExist {
		cm, err := kube.GetConfigMap()
		if err != nil {
			return err
		}

		// Data может придти пустой - нужно создать карту перед добавлением туда значений.
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		cm.Data[fmt.Sprintf("%s-values", ModuleName)] = string(valuesYaml)
		cm.Data[fmt.Sprintf("%s-checksum", ModuleName)] = checksum

		updatedCm, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).Update(cm)
		if err != nil {
			return err
		}
		knownCmResourceVersion = updatedCm.ResourceVersion
	} else {
		cm := v1.ConfigMap{}
		cm.Name = kube.AntiopaConfigMap
		cm.Data = make(map[string]string)

		cm.Data[fmt.Sprintf("%s-values", ModuleName)] = string(valuesYaml)
		cm.Data[fmt.Sprintf("%s-checksum", ModuleName)] = checksum

		updatedCm, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).Create(&cm)
		if err != nil {
			return err
		}
		knownCmResourceVersion = updatedCm.ResourceVersion
	}

	return nil
}

func parseValuesYaml(valuesYamlStr string) (map[interface{}]interface{}, error) {
	res := make(map[interface{}]interface{})

	err := yaml.Unmarshal([]byte(valuesYamlStr), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func getConfigMapModulesValuesYaml(cm *v1.ConfigMap) map[string]string {
	res := make(map[string]string)
	for key, value := range cm.Data {
		if strings.HasSuffix(key, "-values") {
			moduleName := strings.TrimSuffix(key, "-values")
			res[moduleName] = value
		}
	}
	return res
}

func getConfigMapModulesValues(cm *v1.ConfigMap) (map[string]map[interface{}]interface{}, error) {
	res := make(map[string]map[interface{}]interface{})

	for moduleName, valuesYaml := range getConfigMapModulesValuesYaml(cm) {
		moduleValues, err := parseValuesYaml(valuesYaml)
		if err != nil {
			return nil, fmt.Errorf("Bad ConfigMap yaml at key '%s-values': %s:\n%s", moduleName, err, valuesYaml)
		}

		res[moduleName] = moduleValues
	}

	return res, nil
}

func InitKubeValuesManager() (KubeValues, error) {
	rlog.Debug("Init kube values manager")

	kubeModulesValuesChecksums = make(map[string]string)

	KubeValuesUpdated = make(chan KubeValues, 1)
	KubeModuleValuesUpdated = make(chan KubeModuleValuesUpdate, 1)

	var res KubeValues
	res.Values = make(map[interface{}]interface{})
	res.ModulesValues = make(map[string]map[interface{}]interface{})

	cmList, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).List(metav1.ListOptions{})
	if err != nil {
		return KubeValues{}, err
	}
	cmExist := false
	for _, cm := range cmList.Items {
		if cm.ObjectMeta.Name == kube.AntiopaConfigMap {
			cmExist = true
			break
		}
	}

	if cmExist {
		cm, err := kube.GetConfigMap()
		if err != nil {
			return KubeValues{}, err
		}

		if valuesYaml, hasKey := cm.Data["values"]; hasKey {
			res.Values, err = parseValuesYaml(valuesYaml)
			if err != nil {
				return KubeValues{}, fmt.Errorf("Bad ConfigMap yaml at key 'values': %s:\n%s", err, valuesYaml)
			}
			kubeValuesChecksum = utils.CalculateChecksum(valuesYaml)
		}

		for moduleName, valuesYaml := range getConfigMapModulesValuesYaml(cm) {
			moduleValues, err := parseValuesYaml(valuesYaml)
			if err != nil {
				return KubeValues{}, fmt.Errorf("Bad ConfigMap yaml at key '%s-values': %s:\n%s", moduleName, err, valuesYaml)
			}

			res.ModulesValues[moduleName] = moduleValues
			kubeModulesValuesChecksums[moduleName] = utils.CalculateChecksum(valuesYaml)
		}

		knownCmResourceVersion = cm.ResourceVersion
	}

	return res, nil
}

func handleNewCm(cm *v1.ConfigMap) error {
	knownCmResourceVersion = cm.ResourceVersion

	actualValuesChecksum := ""
	valuesYaml, hasValuesKey := cm.Data["values"]
	if hasValuesKey {
		actualValuesChecksum = utils.CalculateChecksum(cm.Data["values"])
	}

	shouldUpdateValues := ((hasValuesKey &&
		actualValuesChecksum != cm.Data["values-checksum"] &&
		actualValuesChecksum != kubeValuesChecksum) ||
		(!hasValuesKey && kubeValuesChecksum != ""))

	if shouldUpdateValues {
		newValues, err := parseValuesYaml(valuesYaml)
		if err != nil {
			return fmt.Errorf("Bad ConfigMap yaml at key 'values': %s:\n%s", err, cm.Data["values"])
		}

		newModulesValues := make(map[string]map[interface{}]interface{})
		newModulesValuesChecksums := make(map[string]string)

		for moduleName, valuesYaml := range getConfigMapModulesValuesYaml(cm) {
			moduleValues, err := parseValuesYaml(valuesYaml)
			if err != nil {
				return fmt.Errorf("Bad ConfigMap yaml at key '%s-values': %s:\n%s", moduleName, err, valuesYaml)
			}

			newModulesValues[moduleName] = moduleValues
			newModulesValuesChecksums[moduleName] = utils.CalculateChecksum(valuesYaml)
		}

		kubeValuesChecksum = actualValuesChecksum
		kubeModulesValuesChecksums = newModulesValuesChecksums

		kubeValuesUpdate := KubeValues{
			Values:        newValues,
			ModulesValues: newModulesValues,
		}

		KubeValuesUpdated <- kubeValuesUpdate
	} else {
		cmModulesValuesYaml := getConfigMapModulesValuesYaml(cm)

		// New modules values and existing modules
		for moduleName, moduleValuesYaml := range cmModulesValuesYaml {
			actualModuleValuesChecksum := utils.CalculateChecksum(moduleValuesYaml)
			if actualModuleValuesChecksum != cm.Data[fmt.Sprintf("%s-values-checksum", moduleName)] && actualModuleValuesChecksum != kubeModulesValuesChecksums[moduleName] {
				moduleValues, err := parseValuesYaml(moduleValuesYaml)
				if err != nil {
					return fmt.Errorf("Bad ConfigMap yaml at key '%s-values': %s:\n%s", moduleName, err, moduleValuesYaml)
				}

				kubeModulesValuesChecksums[moduleName] = actualModuleValuesChecksum

				KubeModuleValuesUpdated <- KubeModuleValuesUpdate{
					ModuleName: moduleName,
					Values:     moduleValues,
				}
			}
		}

		for moduleName := range kubeModulesValuesChecksums {
			if _, hasKey := cmModulesValuesYaml[moduleName]; !hasKey {
				delete(kubeModulesValuesChecksums, moduleName)
				KubeModuleValuesUpdated <- KubeModuleValuesUpdate{
					ModuleName: moduleName,
					Values:     make(map[interface{}]interface{}),
				}
			}
		}
	}

	return nil
}

func handleCmAdd(cm *v1.ConfigMap) error {
	return handleNewCm(cm)
}

func handleCmUpdate(_ *v1.ConfigMap, cm *v1.ConfigMap) error {
	return handleNewCm(cm)
}

func handleCmDelete(cm *v1.ConfigMap) error {
	if kubeValuesChecksum != "" {
		kubeValuesChecksum = ""
		kubeModulesValuesChecksums = make(map[string]string)
		KubeValuesUpdated <- KubeValues{
			Values:        make(map[interface{}]interface{}),
			ModulesValues: make(map[string]map[interface{}]interface{}),
		}
	} else {
		// Global values is already known to be empty.
		// So check each module values change separately.

		updateModulesNames := make([]string, 0)
		for moduleName := range kubeModulesValuesChecksums {
			updateModulesNames = append(updateModulesNames, moduleName)
		}
		for _, moduleName := range updateModulesNames {
			delete(kubeModulesValuesChecksums, moduleName)
			KubeModuleValuesUpdated <- KubeModuleValuesUpdate{
				ModuleName: moduleName,
				Values:     make(map[interface{}]interface{}),
			}
		}
	}

	return nil
}

func RunKubeValuesManager() {
	rlog.Debug("Run kube values manager")

	lw := cache.NewListWatchFromClient(
		kube.KubernetesClient.CoreV1().RESTClient(),
		"configmaps",
		kube.KubernetesAntiopaNamespace,
		fields.OneTermEqualSelector("metadata.name", kube.AntiopaConfigMap))

	cmInformer := cache.NewSharedInformer(
		lw,
		&v1.ConfigMap{},
		time.Duration(15)*time.Second)

	cmInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err := handleCmAdd(obj.(*v1.ConfigMap))
			if err != nil {
				rlog.Errorf("Kube values manager: cannot handle ConfigMap add: %s", err)
			}
		},
		UpdateFunc: func(prevObj interface{}, obj interface{}) {
			err := handleCmUpdate(prevObj.(*v1.ConfigMap), obj.(*v1.ConfigMap))
			if err != nil {
				rlog.Errorf("Kube values manager: cannot handle ConfigMap update: %s", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			err := handleCmDelete(obj.(*v1.ConfigMap))
			if err != nil {
				rlog.Errorf("Kube values manager: cannot handle ConfigMap delete: %s", err)
			}
		},
	})

	cmInformer.Run(make(<-chan struct{}, 1))
}
