package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/evanphx/json-patch"
	"github.com/gobwas/glob"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/kube_values_manager"
	"github.com/deckhouse/deckhouse/antiopa/merge_values"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type Module struct {
	Name          string
	DirectoryName string
	Path          string
}

func (m *Module) isEnabled() (bool, error) {
	enabledScriptPath := filepath.Join(m.DirectoryName, "enabled")

	_, err := os.Stat(enabledScriptPath)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	// TODO: generate and pass enabled modules (modulesOrder)
	cmd := makeCommand(m.Path, "", enabledScriptPath, []string{})
	if err := execCommand(cmd); err != nil {
		return false, err
	}

	return true, nil
}

func RunModules() {
	retryModulesNamesQueue = make([]string, 0)
	for _, moduleName := range modulesOrder {
		RunModule(moduleName)
	}
}

func RunModuleOld(moduleName string) {
	vals, err := PrepareModuleValues(moduleName)
	if err != nil {
		rlog.Error(err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}
	rlog.Debugf("Module '%s': Prepared VALUES:\n%s", moduleName, valuesToString(vals))

	valuesPath, err := dumpModuleValuesYaml(moduleName, vals)
	if err != nil {
		rlog.Errorf("Module '%s': dump values yaml error: %s", moduleName, err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}

	err = CleanupModule(moduleName)
	if err != nil {
		rlog.Error(err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}

	//err = RunModuleBeforeHelmHooks(moduleName, valuesPath)
	//if err != nil {
	//	rlog.Error(err)
	//	retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
	//	return
	//}

	err = RunModuleHelm(moduleName, valuesPath)
	if err != nil {
		rlog.Error(err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
	}

	//err = RunModuleAfterHelmHooks(moduleName, valuesPath)
	//if err != nil {
	//	rlog.Error(err)
	//	retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
	//	return
	//}
}

func RunModuleHelm(moduleName string, ValuesPath string) (err error) {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return fmt.Errorf("Module '%s': no such module", moduleName)
	}

	chartExists, err := CheckModuleHelmChart(module)
	if !chartExists {
		if err != nil {
			rlog.Debugf("Module '%s': helm not needed: %s", module.Name, err)
			return nil
		}
	}

	rlog.Infof("Module '%s': running helm ...", module.Name)

	helmReleaseName := GenerateHelmReleaseName(moduleName)

	err = execCommand(makeCommand(module.Path, ValuesPath, "helm", []string{"upgrade", helmReleaseName, ".", "--install", "--namespace", helm.TillerNamespace, "--values", ValuesPath}))
	if err != nil {
		return fmt.Errorf("Module '%s': helm FAILED: %s", module.Name, err)
	}

	return
}

func CheckModuleHelmChart(module *Module) (chartExists bool, err error) {
	chartPath := filepath.Join(module.Path, "Chart.yaml")

	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		return false, fmt.Errorf("chart file not found '%s'", module.Name, chartPath)
	}
	return true, nil
}

func GenerateHelmReleaseName(moduleName string) string {
	return moduleName
}

func CleanupModule(moduleName string) (err error) {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return fmt.Errorf("Module '%s': no such module", moduleName)
	}

	chartExists, err := CheckModuleHelmChart(module)
	if !chartExists {
		if err != nil {
			rlog.Debugf("Module '%s': cleanup not needed: %s", moduleName, err)
			return nil
		}
	}

	rlog.Infof("Module '%s': running cleanup ...", moduleName)

	helmReleaseName := GenerateHelmReleaseName(moduleName)

	helm.HelmDeleteSingleFailedRevision(helmReleaseName)
	return nil
}

func PrepareModuleValues(moduleName string) (map[interface{}]interface{}, error) {
	if _, hasModule := modulesByName[moduleName]; !hasModule {
		return nil, fmt.Errorf("Module '%s': no such module", moduleName)
	}
	return merge_values.MergeValues(globalConfigValues, globalModulesConfigValues[moduleName], kubeConfigValues, kubeModulesConfigValues[moduleName], dynamicValues, modulesDynamicValues[moduleName]), nil
}

func matchesGlob(value string, globPattern string) bool {
	g, err := glob.Compile(globPattern)
	if err != nil {
		return false
	}
	return g.Match(value)
}

func getEnabledModules() ([]Module, error) {
	allModules, err := readModules()
	if err != nil {
		return nil, err
	}

	cm, err := kube.GetConfigMap()
	if err != nil {
		return nil, err
	}

	var disabledModulesNames []string
	for _, configKey := range []string{"disable-modules", "disabled-modules"} {
		if _, hasKey := cm.Data[configKey]; hasKey {
			disabledModulesNames = make([]string, 0)
			for _, moduleName := range strings.Split(cm.Data[configKey], ",") {
				disabledModulesNames = append(disabledModulesNames, strings.TrimSpace(moduleName))
			}
		}
	}

	for _, disabledModuleName := range disabledModulesNames {
		found := false
		for _, module := range allModules {
			if matchesGlob(module.Name, disabledModuleName) {
				found = true
				break
			}
		}

		if !found {
			rlog.Warnf("Bad value '%s' in antiopa ConfigMap disabled-modules: does not match any module", disabledModuleName)
		}
	}

	res := make([]Module, 0)
	for _, module := range allModules {
		isEnabled := true

		for _, disabledModuleName := range disabledModulesNames {
			if matchesGlob(module.Name, disabledModuleName) {
				isEnabled = false
				break
			}
		}

		if isEnabled {
			res = append(res, module)
		}
	}

	return res, nil
}

func readModules() ([]Module, error) {
	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("cannot list modules directory %s: %s", modulesDir, err)
	}

	var validmoduleName = regexp.MustCompile(`^[0-9][0-9][0-9]-(.*)$`)

	res := make([]Module, 0)
	badModulesDirs := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			matchRes := validmoduleName.FindStringSubmatch(file.Name())
			if matchRes != nil {
				module := Module{
					Name:          matchRes[1],
					DirectoryName: file.Name(),
					Path:          filepath.Join(modulesDir, file.Name()),
				}
				res = append(res, module)
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	if len(badModulesDirs) > 0 {
		return nil, fmt.Errorf("bad module directory names, must match regex `%s`: %s", validmoduleName, strings.Join(badModulesDirs, ", "))
	}

	return res, nil
}

func dumpModuleValuesYaml(moduleName string, values map[interface{}]interface{}) (string, error) {
	return dumpValuesYaml(fmt.Sprintf("%s.yaml", moduleName), values)
}

func readModuleValues(module *Module) (map[interface{}]interface{}, error) {
	path := filepath.Join(module.Path, "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	values, err := readValuesYamlFile(path)
	if err != nil {
		return nil, err
	}
	return values, nil
}

func readModulesValues() (map[interface{}]interface{}, error) {
	path := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[interface{}]interface{}), nil
	}

	return readValuesYamlFile(path)
}

func readValuesYamlFile(filePath string) (map[interface{}]interface{}, error) {
	valuesYaml, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	var res map[interface{}]interface{}

	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return res, nil
}

func makeCommand(dir string, valuesPath string, entrypoint string, args []string) *exec.Cmd {
	envs := make([]string, 0)
	envs = append(envs, os.Environ()...)
	envs = append(envs, helm.CommandEnv()...)
	envs = append(envs, fmt.Sprintf("VALUES_PATH=%s", valuesPath))

	return utils.MakeCommand(dir, entrypoint, args, envs)
}

func getExecutableFilesPaths(dir string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			return nil
		}

		isExecutable := f.Mode()&0111 != 0
		if isExecutable {
			paths = append(paths, path)
		} else {
			rlog.Warnf("Ignoring non executable file %s", filepath.Join(dir, path))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return paths, nil
}

func valuesToString(values map[interface{}]interface{}) string {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return fmt.Sprintf("%v", values)
	}
	return string(valuesYaml)
}

func execCommand(cmd *exec.Cmd) error {
	rlog.Debugf("Executing command in %s: `%s`", cmd.Dir, strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func execCommandOutput(cmd *exec.Cmd) ([]byte, error) {
	rlog.Debugf("Executing command output in %s: `%s`", cmd.Dir, strings.Join(cmd.Args, " "))
	cmd.Stdout = nil
	return cmd.Output()
}

func dumpValuesYaml(fileName string, values map[interface{}]interface{}) (string, error) {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(TempDir, fileName)

	err = ioutil.WriteFile(filePath, valuesYaml, 0644)
	if err != nil {
		return "", err
	}

	return filePath, nil
}
func createResultFile(filePath string) error {
	os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil
	}

	file.Close()
	return nil
}

func readValuesJsonFile(filePath string) (map[string]interface{}, error) {
	valuesJson, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(valuesJson) == 0 {
		return make(map[string]interface{}), nil
	}

	var res map[string]interface{}

	err = json.Unmarshal(valuesJson, &res)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return res, nil
}
func readJsonPatchFile(filePath string) (*jsonpatch.Patch, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	patch, err := jsonpatch.DecodePatch(data)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return &patch, nil
}
func dumpGlobalHooksValuesYaml() (string, error) {
	return dumpValuesYaml("global-hooks.yaml", prepareGlobalValues())
}
func prepareGlobalValues() map[interface{}]interface{} {
	return merge_values.MergeValues(globalConfigValues, kubeConfigValues, dynamicValues)
}
func readValues() (map[interface{}]interface{}, error) {
	path := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[interface{}]interface{}), nil
	}

	return readValuesYamlFile(path)
}
