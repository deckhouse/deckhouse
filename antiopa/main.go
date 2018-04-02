package main

import (
	_ "encoding/json"
	_ "fmt"
	_ "github.com/evanphx/json-patch"
	_ "github.com/romana/rlog"
	_ "gopkg.in/yaml.v2"
	_ "io/ioutil"
	_ "os"
	_ "os/exec"
	_ "path/filepath"
	_ "strings"
	"time"

	_ "github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	_ "github.com/deckhouse/deckhouse/antiopa/helm"
	_ "github.com/deckhouse/deckhouse/antiopa/kube_config_manager"
	_ "github.com/deckhouse/deckhouse/antiopa/kube_node_manager"
	_ "github.com/deckhouse/deckhouse/antiopa/kube_values_manager"
	_ "github.com/deckhouse/deckhouse/antiopa/merge_values"
	_ "github.com/deckhouse/deckhouse/antiopa/module_manager"
	_ "github.com/deckhouse/deckhouse/antiopa/utils"
)

var (
	WorkingDir string
	TempDir    string

	// Имя хоста совпадает с именем пода. Можно использовать для запросов API
	Hostname string
)

func main() {
	// Init()
	// Run()
	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}

// func OnKubeNodeChanged() {
// 	rlog.Infof("Kube node change detected")

// 	if err := RunOnKubeNodeChangedHooks(); err != nil {
// 		rlog.Errorf("on-kube-node-change hooks error: %s", err)
// 		return
// 	}

// 	if valuesChanged {
// 		rlog.Debug("Global values changed: run all modules")
// 		RunModules()
// 	} else {
// 		for _, moduleName := range modulesOrder {
// 			if changed, exist := modulesValuesChanged[moduleName]; exist && changed {
// 				rlog.Debugf("Module `%s` values changed: run module", moduleName)
// 				RunModule(moduleName)
// 			}
// 		}
// 	}

// 	valuesChanged = false
// 	modulesValuesChanged = make(map[string]bool)
// }

// func Init() {
// rlog.Debug("Init")

// var err error

// WorkingDir, err = os.Getwd()
// if err != nil {
// 	rlog.Errorf("MAIN Fatal: Cannot determine antiopa working dir: %s", err)
// 	os.Exit(1)
// }

// TempDir, err = ioutil.TempDir("", "antiopa-")
// if err != nil {
// 	rlog.Errorf("MAIN Fatal: cannot create antiopa temporary dir: %s", err)
// 	os.Exit(1)
// }

// retryModulesNamesQueue = make([]string, 0)
// retryAll = false

// Hostname, err = os.Hostname()
// if err != nil {
// 	rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %v", err)
// 	os.Exit(1)
// }

// InitKube()

// helm.Init(KubernetesAntiopaNamespace)

// // Initialize global enabled-modules index with descriptors
// modules, err := getEnabledModules()
// if err != nil {
// 	rlog.Errorf("Cannot detect enabled antiopa modules: %s", err)
// 	os.Exit(1)
// }
// if len(modules) == 0 {
// 	rlog.Warnf("No modules enabled")
// }
// modulesOrder = make([]string, 0)
// modulesByName = make(map[string]Module)
// for _, module := range modules {
// 	modulesByName[module.Name] = module
// 	modulesOrder = append(modulesOrder, module.Name)
// 	rlog.Debugf("Using module %s", module.Name)
// }

// globalConfigValues, err = readModulesValues()
// if err != nil {
// 	rlog.Errorf("Cannot read values: %s", err)
// 	os.Exit(1)
// }
// rlog.Debugf("Read global VALUES:\n%s", valuesToString(globalConfigValues))

// globalModulesConfigValues = make(map[string]map[interface{}]interface{})
// for _, module := range modulesByName {
// 	values, err := readModuleValues(module)
// 	if err != nil {
// 		rlog.Errorf("Cannot read module %s global values: %s", module.Name, err)
// 		os.Exit(1)
// 	}
// 	if values != nil {
// 		globalModulesConfigValues[module.Name] = values
// 		rlog.Debugf("Read module %s global VALUES:\n%s", module.Name, valuesToString(values))
// 	}
// }

// // TODO: remove InitKubeValuesManager
// res, err := InitKubeValuesManager()
// if err != nil {
// 	rlog.Errorf("Cannot initialize kube values manager: %s", err)
// 	os.Exit(1)
// }
// kubeConfigValues = res.Values
// kubeModulesConfigValues = res.ModulesValues
// rlog.Debugf("Read kube VALUES:\n%s", valuesToString(kubeConfigValues))
// for moduleName, kubeModuleValues := range kubeModulesConfigValues {
// 	rlog.Debugf("Read module %s kube VALUES:\n%s", moduleName, valuesToString(kubeModuleValues))
// }

// config, err := kube_config_manager.Init()
// if err != nil {
// 	rlog.Errorf("Cannot initialize kube config manager: %s", err)
// 	os.Exit(1)
// }
// _ = config
// // TODO: set config

// InitKubeNodeManager()

// err = InitRegistryManager()
// if err != nil {
// 	rlog.Errorf("Cannot initialize registry manager: %s", err)
// 	os.Exit(1)
// }

// dynamicValues = make(map[interface{}]interface{})
// modulesDynamicValues = make(map[string]map[interface{}]interface{})
// modulesValuesChanged = make(map[string]bool)
// }

// func Run() {
// 	rlog.Debug("Run")

// go RunKubeValuesManager()
// go RunKubeNodeManager()
// go RunRegistryManager()

// RunAll()

// retryTicker := time.NewTicker(time.Duration(30) * time.Second)

// for {
// 	select {
// 	case newKubevalues := <-KubeValuesUpdated:
// 		kubeConfigValues = newKubevalues.Values
// 		kubeModulesConfigValues = newKubevalues.ModulesValues

// 		rlog.Infof("Kube values has been updated, rerun all modules ...")

// 		RunModules()

// 	case moduleValuesUpdate := <-KubeModuleValuesUpdated:
// 		if _, hasKey := modulesByName[moduleValuesUpdate.ModuleName]; hasKey {
// 			kubeModulesConfigValues[moduleValuesUpdate.ModuleName] = moduleValuesUpdate.Values

// 			rlog.Infof("Module %s kube values has been updated, rerun ...", moduleValuesUpdate.ModuleName)

// 			RunModule(moduleValuesUpdate.ModuleName)
// 		}

// 	case <-KubeNodeChanged:
// 		OnKubeNodeChanged()

// 	case <-retryTicker.C:
// 		if retryAll {
// 			retryAll = false

// 			rlog.Infof("Retrying all modules ...")

// 			RunAll()
// 		} else if len(retryModulesNamesQueue) > 0 {
// 			retryModuleName := retryModulesNamesQueue[0]
// 			retryModulesNamesQueue = retryModulesNamesQueue[1:]

// 			rlog.Infof("Retrying module %s ...", retryModuleName)

// 			RunModule(retryModuleName)
// 		}

// 	case newImageId := <-ImageUpdated:
// 		err := KubeUpdateDeployment(newImageId)
// 		if err == nil {
// 			rlog.Infof("KUBE deployment update successful, exiting ...")
// 			os.Exit(1)
// 		} else {
// 			rlog.Errorf("KUBE deployment update error: %s", err)
// 		}
// 	}
// }
// }

// func RunAll() {
// if err := RunOnKubeNodeChangedHooks(); err != nil {
// 	retryAll = true
// 	rlog.Errorf("on-kube-node-change hooks error: %s", err)
// 	return
// }

// RunModules()
// }

// Вызов хуков при изменении опций узлов и самих узлов.
// Таким образом можно подтюнить узлы кластера.
// см. `/global-hooks/on-kube-node-change/*`

// func RunOnKubeNodeChangedHooks() error {
// 	globalHooksValuesPath, err := dumpGlobalHooksValuesYaml()
// 	if err != nil {
// 		return err
// 	}

// 	hooksDir := filepath.Join(WorkingDir, "global-hooks", "on-kube-node-change")
// 	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
// 		return nil
// 	}

// 	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
// 	if err != nil {
// 		return err
// 	}

// 	for _, hookName := range hooksNames {
// 		rlog.Infof("Running global on-kube-node-change hook %s ...", hookName)

// 		configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := runGlobalHook(hooksDir, hookName, globalHooksValuesPath)
// 		if err != nil {
// 			return err
// 		}

// 		var kubeConfigValuesChanged, dynamicValuesChanged bool

// 		if kubeConfigValues, kubeConfigValuesChanged, err = ApplyJsonMergeAndPatch(kubeConfigValues, configVJMV, configVJPV); err != nil {
// 			return err
// 		}

// 		if err := SetKubeValues(kubeConfigValues); err != nil {
// 			return err
// 		}

// 		if dynamicValues, dynamicValuesChanged, err = ApplyJsonMergeAndPatch(dynamicValues, dynamicVJMV, dynamicVJPV); err != nil {
// 			return err
// 		}

// 		valuesChanged = valuesChanged || kubeConfigValuesChanged || dynamicValuesChanged
// 	}

// 	return nil
// }
