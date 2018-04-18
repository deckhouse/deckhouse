package module_manager

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/deckhouse/deckhouse/antiopa/utils"

	"github.com/romana/rlog"
)

type Module struct {
	Name          string
	DirectoryName string
	Path          string

	moduleManager *MainModuleManager
}

func (mm *MainModuleManager) NewModule() *Module {
	module := &Module{}
	module.moduleManager = mm
	return module
}

func (m *Module) run() error {
	if err := m.cleanup(); err != nil {
		return err
	}

	if err := m.runHooksByBinding(BeforeHelm); err != nil {
		return err
	}

	if err := m.execRun(); err != nil {
		return err
	}

	if err := m.runHooksByBinding(AfterHelm); err != nil {
		return err
	}

	return nil
}

func (m *Module) cleanup() error {
	chartExists, err := m.checkHelmChart()
	if !chartExists {
		if err != nil {
			rlog.Debugf("Module '%s': cleanup not needed: %s", m.Name, err)
			return nil
		}
	}

	rlog.Infof("Module '%s': running cleanup ...", m.Name)

	if err := m.moduleManager.helm.DeleteSingleFailedRevision(m.generateHelmReleaseName()); err != nil {
		return err
	}

	return nil
}

func (m *Module) execRun() error {
	err := m.execHelm(func(valuesPath, helmReleaseName string) error {
		return m.moduleManager.helm.UpgradeRelease(helmReleaseName, m.Path, []string{valuesPath}, m.moduleManager.helm.TillerNamespace())
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *Module) delete() error {
	if err := m.moduleManager.helm.DeleteRelease(m.generateHelmReleaseName()); err != nil {
		return err
	}

	if err := m.runHooksByBinding(AfterDeleteHelm); err != nil {
		return err
	}

	return nil
}

func (m *Module) execDelete() error {
	err := m.execHelm(func(_, helmReleaseName string) error {
		return m.moduleManager.helm.DeleteRelease(helmReleaseName)
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *Module) execHelm(executeHelm func(valuesPath, helmReleaseName string) error) error {
	chartExists, err := m.checkHelmChart()
	if !chartExists {
		if err != nil {
			rlog.Debugf("Module '%s': helm not needed: %s", m.Name, err)
			return nil
		}
	}

	rlog.Infof("Module '%s': running helm ...", m.Name)

	helmReleaseName := m.generateHelmReleaseName()
	valuesPath, err := m.prepareValuesPath()
	if err != nil {
		return err
	}

	if err = executeHelm(valuesPath, helmReleaseName); err != nil {
		return err
	}

	return nil
}

func (m *Module) runHooksByBinding(binding BindingType) error {
	moduleHooksAfterHelm, err := m.moduleManager.GetModuleHooksInOrder(m.Name, binding)
	if err != nil {
		return err
	}

	for _, moduleHookName := range moduleHooksAfterHelm {
		moduleHook, err := m.moduleManager.GetModuleHook(moduleHookName)
		if err != nil {
			return err
		}

		if err := moduleHook.run(binding); err != nil {
			return err
		}
	}

	return nil
}

func (m *Module) prepareValuesPath() (string, error) {
	values := m.values()

	rlog.Debugf("Prepared module %s values:\n%s", m.Name, utils.ValuesToString(values))

	valuesPath, err := dumpValuesJson(fmt.Sprintf("%s-values.json", m.Name), values)
	if err != nil {
		return "", err
	}
	return valuesPath, nil
}

func (m *Module) prepareConfigValuesPath() (string, error) {
	values := m.configValues()

	rlog.Debugf("Prepared module %s config values:\n%s", m.Name, utils.ValuesToString(values))

	configValuesPath, err := dumpValuesJson(fmt.Sprintf("%s-config-values.json", m.Name), values)
	if err != nil {
		return "", err
	}
	return configValuesPath, nil
}

func (m *Module) prepareDynamicValuesPath() (string, error) {
	values := m.dynamicValues()

	rlog.Debugf("Prepared module %s dynamic values:\n%s", m.Name, utils.ValuesToString(values))

	dynamicValuesPath, err := dumpValuesJson(fmt.Sprintf("%s-dynamic-values.json", m.Name), values)
	if err != nil {
		return "", err
	}
	return dynamicValuesPath, nil
}

func (m *Module) checkHelmChart() (bool, error) {
	chartPath := filepath.Join(m.Path, "Chart.yaml")

	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		return false, fmt.Errorf("module '%s' chart file not found '%s'", m.Name, chartPath)
	}
	return true, nil
}

func (m *Module) generateHelmReleaseName() string {
	return m.Name
}

func (m *Module) values() utils.Values {
	return utils.MergeValues(m.configValues(), m.dynamicValues())
}

func (m *Module) configValues() utils.Values {
	return utils.MergeValues(
		m.moduleManager.globalConfigValues,
		m.moduleManager.kubeGlobalConfigValues,
		m.moduleManager.modulesConfigValues[m.Name],
		m.moduleManager.kubeModulesConfigValues[m.Name],
	)
}

func (m *Module) dynamicValues() utils.Values {
	return utils.MergeValues(m.moduleManager.globalDynamicValues, m.moduleManager.modulesDynamicValues[m.Name])
}

func (m *Module) moduleValuesKey() string {
	return utils.ModuleNameToValuesKey(m.Name)
}

func (m *Module) checkIsEnabledByScript(precedingEnabledModules []string) (bool, error) {
	enabledScriptPath := filepath.Join(m.Path, "enabled")

	f, err := os.Stat(enabledScriptPath)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	if !utils.IsFileExecutable(f) {
		return false, fmt.Errorf("cannot execute non-executable enable script '%s'", enabledScriptPath)
	}

	enabledModulesFilePath, err := dumpValuesJson(fmt.Sprintf("%s-preceding-enabled-modules", m.Name), precedingEnabledModules)
	if err != nil {
		return false, err
	}

	cmd := m.moduleManager.makeCommand(WorkingDir, enabledScriptPath, []string{}, []string{fmt.Sprintf("ENABLED_MODULES_PATH=%s", enabledModulesFilePath)})
	if err := execCommand(cmd); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.Sys().(syscall.WaitStatus).ExitStatus() == 1 {
				return false, nil
			} else {
				return false, err
			}
		} else {
			return false, err
		}
	}

	return true, nil
}

func (mm *MainModuleManager) initModulesIndex() error {
	rlog.Info("Initializing modules ...")

	mm.modulesByName = make(map[string]*Module)
	mm.modulesHooksByName = make(map[string]*ModuleHook)
	mm.modulesHooksOrderByName = make(map[string]map[BindingType][]*ModuleHook)

	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir) // returns a list of modules sorted by filename
	if err != nil {
		return fmt.Errorf("cannot list modules directory '%s': %s", modulesDir, err)
	}

	if err := mm.setGlobalConfigValues(); err != nil {
		return err
	}
	rlog.Debugf("Set mm.configValues:\n%s", utils.ValuesToString(mm.globalConfigValues))

	mm.modulesConfigValues = make(map[string]utils.Values)

	mm.modulesDynamicValues = make(map[string]utils.Values)

	var validModuleName = regexp.MustCompile(`^[0-9][0-9][0-9]-(.*)$`)

	badModulesDirs := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			matchRes := validModuleName.FindStringSubmatch(file.Name())
			if matchRes != nil {
				moduleName := matchRes[1]
				rlog.Infof("Initializing module '%s' ...", moduleName)

				modulePath := filepath.Join(modulesDir, file.Name())

				module := mm.NewModule()
				module.Name = moduleName
				module.DirectoryName = file.Name()
				module.Path = modulePath

				moduleConfig, err := mm.getModuleConfig(module)
				if err != nil {
					return err
				}

				if moduleConfig == nil || moduleConfig.IsEnabled {
					mm.modulesByName[module.Name] = module
					mm.allModuleNamesInOrder = append(mm.allModuleNamesInOrder, module.Name)

					if moduleConfig != nil {
						mm.modulesConfigValues[moduleName] = moduleConfig.Values
						rlog.Debugf("Set modulesConfigValues[%s]:\n%s", moduleName, utils.ValuesToString(mm.modulesConfigValues[moduleName]))
					}

					mm.modulesDynamicValues[moduleName] = make(utils.Values)

					if err = mm.initModuleHooks(module); err != nil {
						return err
					}
				}
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	if len(badModulesDirs) > 0 {
		return fmt.Errorf("bad module directory names, must match regex '%s': %s", validModuleName, strings.Join(badModulesDirs, ", "))
	}

	return nil
}

func (mm *MainModuleManager) setGlobalConfigValues() (err error) {
	values, err := readModulesValues()
	if err != nil {
		return err
	}
	mm.globalConfigValues = values

	return nil
}

func (mm *MainModuleManager) getModuleConfig(module *Module) (*utils.ModuleConfig, error) {
	valuesYamlPath := filepath.Join(module.Path, "values.yaml")

	if _, err := os.Stat(valuesYamlPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(valuesYamlPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read '%s': %s", module.Path, err)
	}

	moduleConfig, err := utils.NewModuleConfigByValuesYamlData(module.Name, data)
	if err != nil {
		return nil, err
	}

	return moduleConfig, nil
}

func readModulesValues() (utils.Values, error) {
	filePath := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return make(utils.Values), nil
	}

	valuesYaml, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read '%s': %s", filePath, err)
	}

	var res map[interface{}]interface{}

	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		return nil, fmt.Errorf("bad '%s': %s\n%s", filePath, err, string(valuesYaml))
	}

	return utils.FormatValues(res)
}

func getExecutableHooksFilesPaths(dir string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			return nil
		}

		if utils.IsFileExecutable(f) {
			paths = append(paths, path)
		} else {
			return fmt.Errorf("found non-executable hook file '%s'", path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return paths, nil
}

func dumpValuesYaml(fileName string, values utils.Values) (string, error) {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(TempDir, fileName)
	if err = dumpData(filePath, valuesYaml); err != nil {
		return "", err
	}

	return filePath, nil
}

func dumpValuesJson(fileName string, values interface{}) (string, error) {
	valuesJson, err := json.Marshal(&values)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(TempDir, fileName)
	if err = dumpData(filePath, valuesJson); err != nil {
		return "", err
	}

	return filePath, nil
}

func dumpData(filePath string, data []byte) error {
	err := ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func execCommand(cmd *exec.Cmd) error {
	rlog.Debugf("Executing command in '%s': '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func (mm *MainModuleManager) makeCommand(dir string, entrypoint string, args []string, envs []string) *exec.Cmd {
	envs = append(envs, os.Environ()...)
	envs = append(envs, mm.helm.CommandEnv()...)
	return utils.MakeCommand(dir, entrypoint, args, envs)
}
