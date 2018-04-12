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
	"github.com/segmentio/go-camelcase"
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
	err := m.execHelm(func(modulePath, valuesPath, helmReleaseName string) []string {
		return []string{
			"upgrade",
			helmReleaseName,
			modulePath,
			"--install",
			"--namespace", m.moduleManager.helm.TillerNamespace(),
			"--values", valuesPath,
		}
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *Module) delete() error {
	if err := m.execDelete(); err != nil {
		return err
	}

	if err := m.runHooksByBinding(AfterDeleteHelm); err != nil {
		return err
	}

	return nil
}

func (m *Module) execDelete() error {
	err := m.execHelm(func(modulePath, valuesPath, helmReleaseName string) []string {
		return []string{
			"delete",
			helmReleaseName,
			"--purge",
			"--namespace", m.moduleManager.helm.TillerNamespace(),
			"--values", valuesPath,
		}
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *Module) execHelm(prepareHelmArgs func(modulePath, valuesPath, helmReleaseName string) []string) error {
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

	cmd := m.moduleManager.makeCommand(WorkingDir, valuesPath, "helm", []string{})
	cmd.Args = prepareHelmArgs(m.Path, valuesPath, helmReleaseName)
	err = execCommand(cmd)
	if err != nil {
		return fmt.Errorf("module '%s': helm FAILED: %s", m.Name, err)
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
	valuesPath, err := dumpValuesYaml(fmt.Sprintf("%s.yaml", m.Name), m.values())
	if err != nil {
		return "", err
	}
	return valuesPath, nil
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
	values := utils.Values{
		"global":          utils.MergeValues(m.moduleManager.globalConfigValues, m.moduleManager.kubeConfigValues, m.moduleManager.dynamicValues),
		m.camelcaseName(): utils.MergeValues(m.moduleManager.globalModulesConfigValues[m.Name], m.moduleManager.kubeModulesConfigValues[m.Name], m.moduleManager.modulesDynamicValues[m.Name]),
	}
	return values
}

func (m *Module) camelcaseName() string {
	return camelcase.Camelcase(m.Name)
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

	cmd := m.moduleManager.makeCommand(WorkingDir, "", enabledScriptPath, []string{})
	cmd.Env = append(cmd.Env, fmt.Sprintf("ENABLED_MODULES_PATH=%s", enabledModulesFilePath))
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
	rlog.Debugf("Set mm.globalConfigValues:\n%s", valuesToString(mm.globalConfigValues))

	mm.globalModulesConfigValues = make(map[string]utils.Values)

	mm.kubeModulesConfigValues = make(map[string]utils.Values) // TODO
	for moduleName, kubeModuleValues := range mm.kubeModulesConfigValues {
		rlog.Debugf("Set kubeModulesConfigValues[%s]:\n%s", moduleName, valuesToString(kubeModuleValues))
	}

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

				moduleConfig, err := mm.getModuleConfig(modulePath)
				if err != nil {
					return err
				}

				if moduleConfig == nil || moduleConfig.IsEnabled {
					mm.modulesByName[module.Name] = module
					mm.allModuleNamesInOrder = append(mm.allModuleNamesInOrder, module.Name)

					if moduleConfig != nil {
						mm.globalModulesConfigValues[moduleName] = moduleConfig.Values
						rlog.Debugf("Set globalModulesConfigValues[%s]:\n%s", moduleName, valuesToString(mm.globalModulesConfigValues[moduleName]))
					}

					mm.kubeModulesConfigValues[moduleName] = make(utils.Values)
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
	mm.globalConfigValues, err = readModulesValues()
	if err != nil {
		return err
	}
	return nil
}

func (mm *MainModuleManager) getModuleConfig(modulePath string) (*utils.ModuleConfig, error) {
	moduleName := filepath.Base(modulePath)
	valuesYamlPath := filepath.Join(modulePath, "values.yaml")

	if _, err := os.Stat(valuesYamlPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(valuesYamlPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read '%s': %s", modulePath, err)
	}

	moduleConfig, err := utils.NewModuleConfigByYamlData(moduleName, data)
	if err != nil {
		return nil, err
	}

	return moduleConfig, nil
}

func readModulesValues() (utils.Values, error) {
	path := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(utils.Values), nil
	}
	return readValuesYamlFile(path)
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

func readValuesYamlFile(filePath string) (utils.Values, error) {
	valuesYaml, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read '%s': %s", filePath, err)
	}

	var res map[interface{}]interface{}

	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		return nil, fmt.Errorf("bad '%s': %s", filePath, err)
	}

	values, err := utils.FormatValues(res)
	if err != nil {
		return nil, err
	}

	return values, nil
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

func valuesToString(values utils.Values) string {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return fmt.Sprintf("%v", values)
	}
	return string(valuesYaml)
}

func execCommand(cmd *exec.Cmd) error {
	rlog.Debugf("Executing command in '%s': '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	return cmd.Run()
}
