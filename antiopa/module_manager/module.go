package module_manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kennygrant/sanitize"
	"github.com/otiai10/copy"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/antiopa/executor"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type Module struct {
	Name          string
	DirectoryName string
	Path          string
	StaticConfig  *utils.ModuleConfig

	moduleManager *MainModuleManager
}

func (mm *MainModuleManager) NewModule() *Module {
	module := &Module{}
	module.moduleManager = mm
	return module
}

func (m *Module) SafeName() string {
	return sanitize.BaseName(m.Name)
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
			rlog.Debugf("MODULE '%s': cleanup not needed: %s", m.Name, err)
			return nil
		}
	}

	//rlog.Infof("MODULE '%s': cleanup helm revisions...", m.Name)
	if err := m.moduleManager.helm.DeleteSingleFailedRevision(m.generateHelmReleaseName()); err != nil {
		return err
	}

	if err := m.moduleManager.helm.DeleteOldFailedRevisions(m.generateHelmReleaseName()); err != nil {
		return err
	}

	return nil
}

func (m *Module) execRun() error {
	err := m.execHelm(func(valuesPath, helmReleaseName string) error {
		var err error

		runChartPath := filepath.Join(TempDir, fmt.Sprintf("%s.chart", m.SafeName()))

		err = os.RemoveAll(runChartPath)
		if err != nil {
			return err
		}
		err = copy.Copy(m.Path, runChartPath)
		if err != nil {
			return err
		}

		// Prepare dummy empty values.yaml for helm not to fail
		err = os.Truncate(filepath.Join(runChartPath, "values.yaml"), 0)
		if err != nil {
			return err
		}

		checksum, err := utils.CalculateChecksumOfPaths(runChartPath, valuesPath)
		if err != nil {
			return err
		}

		doRelease := true

		isReleaseExists, err := m.moduleManager.helm.IsReleaseExists(helmReleaseName)
		if err != nil {
			return err
		}

		if isReleaseExists {
			_, status, err := m.moduleManager.helm.LastReleaseStatus(helmReleaseName)
			if err != nil {
				return err
			}

			// Skip helm release for unchanged modules only for non FAILED releases
			if status != "FAILED" {
				releaseValues, err := m.moduleManager.helm.GetReleaseValues(helmReleaseName)
				if err != nil {
					return err
				}

				if recordedChecksum, hasKey := releaseValues["_antiopaModuleChecksum"]; hasKey {
					if recordedChecksumStr, ok := recordedChecksum.(string); ok {
						if recordedChecksumStr == checksum {
							doRelease = false
							rlog.Infof("MODULE_RUN '%s': helm release '%s' checksum '%s' does not changed: skip helm upgrade", m.Name, helmReleaseName, checksum)
						} else {
							rlog.Debugf("MODULE_RUN '%s': helm release '%s' checksum changed '%s' -> '%s': upgrade helm release", m.Name, helmReleaseName, recordedChecksumStr, checksum)
						}
					}
				}
			}
		}

		if doRelease {
			rlog.Debugf("MODULE_RUN '%s': helm release '%s' checksum '%s': installing/upgrading release", m.Name, helmReleaseName, checksum)

			return m.moduleManager.helm.UpgradeRelease(
				helmReleaseName, runChartPath,
				[]string{valuesPath},
				[]string{fmt.Sprintf("_antiopaModuleChecksum=%s", checksum)},
				m.moduleManager.helm.TillerNamespace(),
			)
		} else {
			rlog.Debugf("MODULE_RUN '%s': helm release '%s' checksum '%s': release install/upgrade is skipped", m.Name, helmReleaseName, checksum)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *Module) delete() error {
	// Если есть chart, но нет релиза — warning
	// если нет чарта — молча перейти к хукам
	// если есть и chart и релиз — удалить
	chartExists, _ := m.checkHelmChart()
	if chartExists {
		releaseExists, err := m.moduleManager.helm.IsReleaseExists(m.generateHelmReleaseName())
		if !releaseExists {
			if err != nil {
				rlog.Warnf("Module delete: Cannot find helm release '%s' for module '%s'. Helm error: %s", m.generateHelmReleaseName(), m.Name, err)
			} else {
				rlog.Warnf("Module delete: Cannot find helm release '%s' for module '%s'.", m.generateHelmReleaseName(), m.Name)
			}
		} else {
			// Есть чарт и есть релиз — запуск удаления
			err := m.moduleManager.helm.DeleteRelease(m.generateHelmReleaseName())
			if err != nil {
				return err
			}
		}
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

	helmReleaseName := m.generateHelmReleaseName()
	valuesPath, err := m.prepareValuesYamlFile()
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

		if err := moduleHook.run(binding, BindingContext{Binding: ContextBindingType[binding]}); err != nil {
			return err
		}
	}

	return nil
}

func (m *Module) prepareConfigValuesYamlFile() (string, error) {
	values := m.configValues()

	data := utils.MustDump(utils.DumpValuesYaml(values))
	path := filepath.Join(TempDir, fmt.Sprintf("%s.module-config-values.yaml", m.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared module %s config values:\n%s", m.Name, utils.ValuesToString(values))

	return path, nil
}

func (m *Module) prepareConfigValuesJsonFile() (string, error) {
	values := m.configValues()

	data := utils.MustDump(utils.DumpValuesJson(values))
	path := filepath.Join(TempDir, fmt.Sprintf("%s.module-config-values.json", m.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared module %s config values:\n%s", m.Name, utils.ValuesToString(values))

	return path, nil
}

func (m *Module) prepareValuesYamlFile() (string, error) {
	values := m.values()

	data := utils.MustDump(utils.DumpValuesYaml(values))
	path := filepath.Join(TempDir, fmt.Sprintf("%s.module-values.yaml", m.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared module %s values:\n%s", m.Name, utils.ValuesToString(values))

	return path, nil
}

func (m *Module) prepareValuesJsonFileWith(values utils.Values) (string, error) {
	data := utils.MustDump(utils.DumpValuesJson(values))
	path := filepath.Join(TempDir, fmt.Sprintf("%s.module-values.json", m.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared module %s values:\n%s", m.Name, utils.ValuesToString(values))

	return path, nil
}

func (m *Module) prepareValuesJsonFile() (string, error) {
	return m.prepareValuesJsonFileWith(m.values())
}

func (m *Module) prepareValuesJsonFileForEnabledScript(precedingEnabledModules []string) (string, error) {
	return m.prepareValuesJsonFileWith(m.valuesForEnabledScript(precedingEnabledModules))
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

// configValues returns values from ConfigMap: global section and module section
func (m *Module) configValues() utils.Values {
	return utils.MergeValues(
		// global section
		utils.Values{"global": map[string]interface{}{}},
		m.moduleManager.kubeGlobalConfigValues,
		// module section
		utils.Values{utils.ModuleNameToValuesKey(m.Name): map[string]interface{}{}},
		m.moduleManager.kubeModulesConfigValues[m.Name],
	)
}

// constructValues returns effective values for module hook:
//
// global: static + kube + patches from hooks
//
// module: static + kube + patches from hooks
//
// global section also contains enabledModules key with previously enabled modules
func (m *Module) constructValues(enabledModules []string) utils.Values {
	var err error

	res := utils.MergeValues(
		// global
		utils.Values{"global": map[string]interface{}{}},
		m.moduleManager.globalStaticValues,
		m.moduleManager.kubeGlobalConfigValues,
		// module
		utils.Values{utils.ModuleNameToValuesKey(m.Name): map[string]interface{}{}},
		m.StaticConfig.Values,
		m.moduleManager.kubeModulesConfigValues[m.Name],
	)

	for _, patches := range [][]utils.ValuesPatch{
		m.moduleManager.globalDynamicValuesPatches,
		m.moduleManager.modulesDynamicValuesPatches[m.Name],
	} {
		for _, patch := range patches {
			// Invariant: do not store patches that does not apply
			// Give user error for patches early, after patch receive

			res, _, err = utils.ApplyValuesPatch(res, patch)
			if err != nil {
				panic(err)
			}
		}
	}

	res = utils.MergeValues(res, m.constructEnabledModulesValues(enabledModules))

	return res
}

func (m *Module) constructEnabledModulesValues(enabledModules []string) utils.Values {
	return utils.Values{
		"global": map[string]interface{}{
			"enabledModules": enabledModules,
		},
	}
}

func (m *Module) valuesForEnabledScript(precedingEnabledModules []string) utils.Values {
	return m.constructValues(precedingEnabledModules)
}

func (m *Module) values() utils.Values {
	return m.constructValues(m.moduleManager.enabledModulesInOrder)
}

func (m *Module) moduleValuesKey() string {
	return utils.ModuleNameToValuesKey(m.Name)
}

func (m *Module) prepareModuleEnabledResultFile() (string, error) {
	path := filepath.Join(TempDir, fmt.Sprintf("%s.module-enabled-result", m.Name))
	if err := createHookResultValuesFile(path); err != nil {
		return "", err
	}
	return path, nil
}

func (m *Module) readModuleEnabledResult(filePath string) (bool, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	value := strings.TrimSpace(string(data))

	if value == "true" {
		return true, nil
	} else if value == "false" {
		return false, nil
	}

	return false, fmt.Errorf("expected 'true' or 'false', got '%s'", value)
}

func (m *Module) checkIsEnabledByScript(precedingEnabledModules []string) (bool, error) {
	enabledScriptPath := filepath.Join(m.Path, "enabled")

	f, err := os.Stat(enabledScriptPath)
	if os.IsNotExist(err) {
		rlog.Debugf("MODULE '%s':  ENABLED. Enabled script is not exist!", m.Name)
		return true, nil
	} else if err != nil {
		return false, err
	}

	if !utils.IsFileExecutable(f) {
		return false, fmt.Errorf("cannot execute non-executable enable script '%s'", enabledScriptPath)
	}

	configValuesPath, err := m.prepareConfigValuesJsonFile()
	if err != nil {
		return false, err
	}

	valuesPath, err := m.prepareValuesJsonFileForEnabledScript(precedingEnabledModules)
	if err != nil {
		return false, err
	}

	enabledResultFilePath, err := m.prepareModuleEnabledResultFile()
	if err != nil {
		return false, err
	}

	rlog.Infof("MODULE '%s': run enabled script '%s'...", m.Name, enabledScriptPath)

	cmd := m.moduleManager.makeHookCommand(
		WorkingDir, configValuesPath, valuesPath, "", enabledScriptPath, []string{},
		[]string{
			fmt.Sprintf("MODULE_ENABLED_RESULT=%s", enabledResultFilePath),
		},
	)

	if err := executor.Run(cmd, true); err != nil {
		return false, err
	}

	moduleEnabled, err := m.readModuleEnabledResult(enabledResultFilePath)
	if err != nil {
		return false, fmt.Errorf("bad enabled result in file MODULE_ENABLED_RESULT=\"%s\" from enabled script '%s' for module '%s': %s", enabledResultFilePath, enabledScriptPath, m.Name, err)
	}

	if moduleEnabled {
		rlog.Debugf("Module '%s'  ENABLED with script. Preceding: %s", m.Name, precedingEnabledModules)
		return true, nil
	}

	rlog.Debugf("Module '%s' DISABLED with script. Preceding: %s ", m.Name, precedingEnabledModules)
	return false, nil
}

// initModulesIndex load all available modules from modules directory
//
func (mm *MainModuleManager) initModulesIndex() error {
	rlog.Info("Initializing modules ...")

	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir) // returns a list of modules sorted by filename
	if err != nil {
		return fmt.Errorf("cannot list modules directory '%s': %s", modulesDir, err)
	}

	if err := mm.initGlobalConfigValues(); err != nil {
		return err
	}
	rlog.Debugf("Set mm.configValues:\n%s", utils.ValuesToString(mm.globalStaticValues))

	var validModuleName = regexp.MustCompile(`^[0-9][0-9][0-9]-(.*)$`)

	badModulesDirs := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			matchRes := validModuleName.FindStringSubmatch(file.Name())
			if matchRes != nil {
				moduleName := matchRes[1]
				rlog.Infof("Load and register module '%s' ...", moduleName)

				modulePath := filepath.Join(modulesDir, file.Name())

				module := mm.NewModule()
				module.Name = moduleName
				module.DirectoryName = file.Name()
				module.Path = modulePath

				// load config from values.yaml
				err := module.loadStaticValues()
				if err != nil {
					return err
				}

				mm.allModulesByName[module.Name] = module
				mm.allModulesNamesInOrder = append(mm.allModulesNamesInOrder, module.Name)
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	rlog.Debugf("initModulesIndex: %v", mm.allModulesByName)

	if len(badModulesDirs) > 0 {
		return fmt.Errorf("bad module directory names, must match regex '%s': %s", validModuleName, strings.Join(badModulesDirs, ", "))
	}

	return nil
}

func (mm *MainModuleManager) initGlobalConfigValues() (err error) {
	values, err := loadGlobalModulesValues()
	if err != nil {
		return
	}
	mm.globalStaticValues = values

	rlog.Debugf("Initialized global static values:\n%s", utils.ValuesToString(mm.globalStaticValues))

	return
}

// loadStaticValues loads config for module from values.yaml
// Module is considered as enabled if values.yaml is not exists.
func (m *Module) loadStaticValues() error {
	valuesYamlPath := filepath.Join(m.Path, "values.yaml")

	if _, err := os.Stat(valuesYamlPath); os.IsNotExist(err) {
		m.StaticConfig = utils.NewModuleConfig(m.Name).WithEnabled(true)
		rlog.Debugf("module %s is enabled: no values.yaml exists", m.Name)
		return nil
	}

	data, err := ioutil.ReadFile(valuesYamlPath)
	if err != nil {
		return fmt.Errorf("cannot read '%s': %s", m.Path, err)
	}

	m.StaticConfig, err = utils.NewModuleConfig(m.Name).FromYaml(data)
	if err != nil {
		return err
	}
	rlog.Debugf("module %s static values: %s", m.Name, utils.ValuesToString(m.StaticConfig.Values))
	return nil
}

func loadGlobalModulesValues() (utils.Values, error) {
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

func (mm *MainModuleManager) makeCommand(dir string, entrypoint string, args []string, envs []string) *exec.Cmd {
	envs = append(envs, os.Environ()...)
	envs = append(envs, mm.helm.CommandEnv()...)
	return utils.MakeCommand(dir, entrypoint, args, envs)
}
