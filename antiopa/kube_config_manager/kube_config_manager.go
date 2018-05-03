package kube_config_manager

import (
	"fmt"
	"github.com/romana/rlog"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/utils"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

const (
	ConfigMapName             = "antiopa"
	ValuesChecksumsAnnotation = "antiopa/values-checksums"
)

type KubeConfigManager interface {
	SetKubeGlobalValues(values utils.Values) error
	SetKubeModuleValues(moduleName string, values utils.Values) error
	Run()
	InitialConfig() *Config
}

type MainKubeConfigManager struct {
	initialConfig *Config

	GlobalValuesChecksum  string
	ModulesValuesChecksum map[string]string
}

type ModuleConfigs map[string]utils.ModuleConfig

type Config struct {
	Values        utils.Values
	ModuleConfigs ModuleConfigs
}

func NewConfig() *Config {
	return &Config{
		Values:        make(utils.Values),
		ModuleConfigs: make(map[string]utils.ModuleConfig),
	}
}

var (
	ConfigUpdated        chan Config
	ModuleConfigsUpdated chan ModuleConfigs
)

func simpleMergeConfigMapData(data map[string]string, newData map[string]string) map[string]string {
	for k, v := range newData {
		data[k] = v
	}
	return data
}

func (kcm *MainKubeConfigManager) saveGlobalKubeConfig(globalKubeConfig GlobalKubeConfig) error {
	return kcm.changeOrCreateKubeConfig(func(obj *v1.ConfigMap) error {
		checksums, err := kcm.getValuesChecksums(obj)
		if err != nil {
			return err
		}

		checksums[utils.GlobalValuesKey] = globalKubeConfig.Checksum

		kcm.setValuesChecksums(obj, checksums)

		obj.Data = simpleMergeConfigMapData(obj.Data, globalKubeConfig.ConfigData)

		return nil
	})
}

func (kcm *MainKubeConfigManager) saveModuleKubeConfig(moduleKubeConfig ModuleKubeConfig) error {
	return kcm.changeOrCreateKubeConfig(func(obj *v1.ConfigMap) error {
		checksums, err := kcm.getValuesChecksums(obj)
		if err != nil {
			return err
		}

		checksums[moduleKubeConfig.ModuleName] = moduleKubeConfig.Checksum

		kcm.setValuesChecksums(obj, checksums)

		obj.Data = simpleMergeConfigMapData(obj.Data, moduleKubeConfig.ConfigData)

		return nil
	})
}

func (kcm *MainKubeConfigManager) changeOrCreateKubeConfig(configChangeFunc func(*v1.ConfigMap) error) error {
	var err error

	obj, err := kcm.getConfigMap()
	if err != nil {
		return nil
	}

	if obj != nil {
		if obj.Data == nil {
			obj.Data = make(map[string]string)
		}

		err = configChangeFunc(obj)
		if err != nil {
			return err
		}

		_, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).Update(obj)
		if err != nil {
			return err
		}

		return nil
	} else {
		obj := &v1.ConfigMap{}
		obj.Name = ConfigMapName
		obj.Data = make(map[string]string)

		err = configChangeFunc(obj)
		if err != nil {
			return err
		}

		_, err := kube.KubernetesClient.CoreV1().ConfigMaps(kube.KubernetesAntiopaNamespace).Create(obj)
		if err != nil {
			return err
		}

		return nil
	}
}

func (kcm *MainKubeConfigManager) SetKubeGlobalValues(values utils.Values) error {
	globalKubeConfig := GetGlobalKubeConfigFromValues(values)

	if globalKubeConfig != nil {
		rlog.Debugf("Kube config manager: set kube global values:\n%s", utils.ValuesToString(values))

		err := kcm.saveGlobalKubeConfig(*globalKubeConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (kcm *MainKubeConfigManager) SetKubeModuleValues(moduleName string, values utils.Values) error {
	moduleKubeConfig := GetModuleKubeConfigFromValues(moduleName, values)

	if moduleKubeConfig != nil {
		rlog.Debugf("Kube config manager: set kube module values:\n%s", moduleKubeConfig.ModuleConfig.String())

		err := kcm.saveModuleKubeConfig(*moduleKubeConfig)
		if err != nil {
			return err
		}
	}

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

func NewMainKubeConfigManager() *MainKubeConfigManager {
	kcm := &MainKubeConfigManager{}
	kcm.initialConfig = NewConfig()
	return kcm
}

func (kcm *MainKubeConfigManager) initConfig() error {
	obj, err := kcm.getConfigMap()
	if err != nil {
		return err
	}

	if obj == nil {
		return nil
	}

	initialConfig := NewConfig()
	globalValuesChecksum := ""
	modulesValuesChecksum := make(map[string]string)

	globalKubeConfig, err := GetGlobalKubeConfigFromConfigData(obj.Data)
	if err != nil {
		return err
	}
	if globalKubeConfig != nil {
		initialConfig.Values = globalKubeConfig.Values
		globalValuesChecksum = globalKubeConfig.Checksum
	}

	for _, module := range GetModulesNamesFromConfigData(obj.Data) {
		// all GetModulesNamesFromConfigData must exist
		moduleKubeConfig, err := ModuleKubeConfigMustExist(GetModuleKubeConfigFromConfigData(module, obj.Data))
		if err != nil {
			return err
		}

		initialConfig.ModuleConfigs[moduleKubeConfig.ModuleName] = moduleKubeConfig.ModuleConfig
		modulesValuesChecksum[moduleKubeConfig.ModuleName] = moduleKubeConfig.Checksum
	}

	kcm.initialConfig = initialConfig
	kcm.GlobalValuesChecksum = globalValuesChecksum
	kcm.ModulesValuesChecksum = modulesValuesChecksum

	return nil
}

func Init() (KubeConfigManager, error) {
	rlog.Debug("Init kube config manager")

	ConfigUpdated = make(chan Config, 1)
	ModuleConfigsUpdated = make(chan ModuleConfigs, 1)

	kcm := NewMainKubeConfigManager()

	err := kcm.initConfig()
	if err != nil {
		return nil, err
	}

	return kcm, nil
}

func (kcm *MainKubeConfigManager) getValuesChecksums(cm *v1.ConfigMap) (map[string]string, error) {
	data, hasKey := cm.Annotations[ValuesChecksumsAnnotation]
	if !hasKey {
		return make(map[string]string), nil
	}

	var res map[string]string
	err := json.Unmarshal([]byte(data), &res)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal json annotation 'antiopa/values-checksums' in ConfigMap '%s': %s\n%s", cm.Name, err, data)
	}

	return res, nil
}

func (kcm *MainKubeConfigManager) setValuesChecksums(cm *v1.ConfigMap, checksums map[string]string) {
	data, err := json.Marshal(checksums)
	if err != nil {
		// nothing should go wrong
		panic(err)
	}

	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[ValuesChecksumsAnnotation] = string(data)
}

func (kcm *MainKubeConfigManager) handleNewCm(obj *v1.ConfigMap) error {
	savedChecksums, err := kcm.getValuesChecksums(obj)
	if err != nil {
		return err
	}

	globalKubeConfig, err := GetGlobalKubeConfigFromConfigData(obj.Data)
	if err != nil {
		return err
	}

	shouldUpdateValues := (globalKubeConfig != nil &&
		globalKubeConfig.Checksum != savedChecksums[utils.GlobalValuesKey] &&
		globalKubeConfig.Checksum != kcm.GlobalValuesChecksum) ||
		(globalKubeConfig == nil && kcm.GlobalValuesChecksum != "")

	if shouldUpdateValues {
		newConfig := NewConfig()
		newGlobalValuesChecksum := ""
		newModulesValuesChecksum := make(map[string]string)

		if globalKubeConfig != nil {
			newConfig.Values = globalKubeConfig.Values
			newGlobalValuesChecksum = globalKubeConfig.Checksum
		}

		for _, module := range GetModulesNamesFromConfigData(obj.Data) {
			// all GetModulesNamesFromConfigData must exist
			moduleKubeConfig, err := ModuleKubeConfigMustExist(GetModuleKubeConfigFromConfigData(module, obj.Data))
			if err != nil {
				return err
			}

			newConfig.ModuleConfigs[moduleKubeConfig.ModuleName] = moduleKubeConfig.ModuleConfig
			newModulesValuesChecksum[moduleKubeConfig.ModuleName] = moduleKubeConfig.Checksum
		}

		kcm.GlobalValuesChecksum = newGlobalValuesChecksum
		kcm.ModulesValuesChecksum = newModulesValuesChecksum

		rlog.Debugf("Kube config manager: got kube global config update:")
		rlog.Debug(utils.ValuesToString(newConfig.Values))
		for _, moduleConfig := range newConfig.ModuleConfigs {
			rlog.Debugf("%s", moduleConfig.String())
		}
		ConfigUpdated <- *newConfig
	} else {
		actualModulesNames := GetModulesNamesFromConfigData(obj.Data)

		moduleConfigsUpdate := make(ModuleConfigs)

		for _, module := range actualModulesNames {
			// all GetModulesNamesFromConfigData must exist
			moduleKubeConfig, err := ModuleKubeConfigMustExist(GetModuleKubeConfigFromConfigData(module, obj.Data))
			if err != nil {
				return err
			}

			if moduleKubeConfig.Checksum != savedChecksums[module] && moduleKubeConfig.Checksum != kcm.ModulesValuesChecksum[module] {
				kcm.ModulesValuesChecksum[module] = moduleKubeConfig.Checksum
				moduleConfigsUpdate[module] = moduleKubeConfig.ModuleConfig
			}
		}

	SearchModulesWithDeletedConfig:
		for module := range kcm.ModulesValuesChecksum {
			for _, actualModule := range actualModulesNames {
				if actualModule == module {
					continue SearchModulesWithDeletedConfig
				}
			}

			delete(kcm.ModulesValuesChecksum, module)

			moduleConfigsUpdate[module] = *utils.NewEmptyModuleConfig(module)
		}

		if len(moduleConfigsUpdate) > 0 {
			rlog.Debugf("Kube config manager: got kube modules configs update:")
			for _, moduleConfig := range moduleConfigsUpdate {
				rlog.Debugf("%s", moduleConfig.String())
			}
			ModuleConfigsUpdated <- moduleConfigsUpdate
		}
	}

	return nil
}

func (kcm *MainKubeConfigManager) handleCmAdd(obj *v1.ConfigMap) error {
	objYaml, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	rlog.Debugf("Kube config manager: informer: handle ConfigMap '%s' add:\n%s", obj.Name, objYaml)

	return kcm.handleNewCm(obj)
}

func (kcm *MainKubeConfigManager) handleCmUpdate(_ *v1.ConfigMap, obj *v1.ConfigMap) error {
	objYaml, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	rlog.Debugf("Kube config manager: informer: handle ConfigMap '%s' update:\n%s", obj.Name, objYaml)

	return kcm.handleNewCm(obj)
}

func (kcm *MainKubeConfigManager) handleCmDelete(obj *v1.ConfigMap) error {
	objYaml, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	rlog.Debugf("Kube config manager: handle ConfigMap '%s' delete:\n%s", obj.Name, objYaml)

	if kcm.GlobalValuesChecksum != "" {
		kcm.GlobalValuesChecksum = ""
		kcm.ModulesValuesChecksum = make(map[string]string)

		ConfigUpdated <- Config{
			Values:        make(utils.Values),
			ModuleConfigs: make(map[string]utils.ModuleConfig),
		}
	} else {
		// Global values is already known to be empty.
		// So check each module values change separately,
		// and generate signals per-module.

		moduleConfigsUpdate := make(ModuleConfigs)

		updateModulesNames := make([]string, 0)
		for module := range kcm.ModulesValuesChecksum {
			updateModulesNames = append(updateModulesNames, module)
		}
		for _, module := range updateModulesNames {
			delete(kcm.ModulesValuesChecksum, module)
			moduleConfigsUpdate[module] = utils.ModuleConfig{
				ModuleName: module,
				IsEnabled:  true,
				Values:     make(utils.Values),
			}
		}

		ModuleConfigsUpdated <- moduleConfigsUpdate
	}

	return nil
}

func (kcm *MainKubeConfigManager) Run() {
	rlog.Debugf("Run kube config manager")

	lw := cache.NewListWatchFromClient(
		kube.KubernetesClient.CoreV1().RESTClient(),
		"configmaps",
		kube.KubernetesAntiopaNamespace,
		fields.OneTermEqualSelector("metadata.name", ConfigMapName))

	cmInformer := cache.NewSharedInformer(lw,
		&v1.ConfigMap{},
		time.Duration(15)*time.Second)

	cmInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err := kcm.handleCmAdd(obj.(*v1.ConfigMap))
			if err != nil {
				rlog.Errorf("Kube config manager: cannot handle ConfigMap add: %s", err)
			}
		},
		UpdateFunc: func(prevObj interface{}, obj interface{}) {
			err := kcm.handleCmUpdate(prevObj.(*v1.ConfigMap), obj.(*v1.ConfigMap))
			if err != nil {
				rlog.Errorf("Kube config manager: cannot handle ConfigMap update: %s", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			err := kcm.handleCmDelete(obj.(*v1.ConfigMap))
			if err != nil {
				rlog.Errorf("Kube config manager: cannot handle ConfigMap delete: %s", err)
			}
		},
	})

	cmInformer.Run(make(<-chan struct{}, 1))
}
