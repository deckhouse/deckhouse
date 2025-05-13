/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alerttemplates

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"tools/helm_generate/helper"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
)

// ../deckhouse/tools/helm_generate/runners/alert_templates/template_values/[module-name].yaml
const userDefinedValuesPath = "tools/helm_generate/runners/alert_templates/template_values"

// ../deckhouse/modules/[module-name]/monitoring/prometheus-rules[template-name].yaml or .tpl
const prometheusRules = "monitoring/prometheus-rules"

// ../deckhouse/modules/[module-name]/templates/_[helper-name].yaml or .tpl
const helpersPath = "templates"

// Deckhouse edition type
const (
	ce     edition = "ce"      // community edition
	ee     edition = "ee"      // enterprise edition
	be     edition = "be"      // basic edition
	se     edition = "se"      // standard edition
	sePlus edition = "se-plus" // standard plus edition
	fe     edition = "fe"      // fan edition
)

var deckhouseRoot = ""

type edition string

type module struct {
	Name    string
	Path    string
	Edition edition
}

type moduleAlert struct {
	Name         string `yaml:"name"`
	SourceFile   string `yaml:"sourceFile"`
	ModuleUrl    string `yaml:"moduleUrl"`
	Module       string `yaml:"module"`
	Edition      string `yaml:"edition"`
	Description  string `yaml:"description"`
	Summary      string `yaml:"summary"`
	Severity     string `yaml:"severity"`
	MarkupFormat string `yaml:"markupFormat"`
}

// Function to check if a value exists in a slice
func containsString(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func stripHTMLTags(input string) string {
	re := regexp.MustCompile(`<.*?>`)
	return re.ReplaceAllString(input, "")
}

func run() error {
	var err error
	d8Alerts := []moduleAlert{}
	d8ModulesWithAlerts := []string{}

	deckhouseRoot, err = helper.DeckhouseRoot()
	if err != nil {
		return err
	}

	for _, module := range modules(deckhouseRoot) {
		yamlTemplates, tplTemplates := moduleTemplates(module)

		moduleUrlName := module.Name
		moduleName := module.Name
		if substr := strings.SplitN(moduleName, "-", 2); len(substr) > 1 {
			moduleName = substr[1]
		}

		if len(yamlTemplates) > 0 {
			for templateName, templateContent := range yamlTemplates {
				templateAlerts, err := getAlertsFromTemplate(templateContent, moduleName, moduleUrlName, string(module.Edition), filepath.Join(module.Path, prometheusRules, templateName))
				if err != nil {
					return err
				}

				d8Alerts = append(d8Alerts, templateAlerts...)

				if !containsString(d8ModulesWithAlerts, moduleName) {
					d8ModulesWithAlerts = append(d8ModulesWithAlerts, moduleName)
				}
			}
		}

		if len(tplTemplates) > 0 {
			tplRelativePaths := helper.GetMapKeys(tplTemplates)
			renderContent, err := renderHelmTemplate(module, tplRelativePaths)
			if err != nil {
				return err
			}

			for _, templatePath := range tplRelativePaths {
				// templatePath may contains subdirectory or not, e.g. "image-availability/image-checks.tpl", "nat-instance.tpl"
				pathSegments := strings.Split(templatePath, "/")
				name := pathSegments[len(pathSegments)-1]

				templateContent := renderContent[fmt.Sprintf("renderdir/templates/%s", name)]

				templateAlerts, err := getAlertsFromTemplate([]byte(templateContent), moduleName, moduleUrlName, string(module.Edition), filepath.Join(module.Path, prometheusRules, templatePath))
				if err != nil {
					return err
				}

				d8Alerts = append(d8Alerts, templateAlerts...)
				if !containsString(d8ModulesWithAlerts, moduleName) {
					d8ModulesWithAlerts = append(d8ModulesWithAlerts, moduleName)
				}
			}
		}
	}

	slices.SortFunc(d8Alerts, func(a, b moduleAlert) int {
		return cmp.Compare(strings.ToLower(fmt.Sprintf("%s %s %s %s", a.Name, a.Module, a.Severity, a.Description)), strings.ToLower(fmt.Sprintf("%s %s %s %s", b.Name, b.Module, b.Severity, b.Description)))
	})
	sort.Strings(d8ModulesWithAlerts)

	data := map[string]interface{}{
		"alerts":                d8Alerts,
		"modules-having-alerts": d8ModulesWithAlerts,
	}
	d8AlertsDataYAML, _ := yaml.Marshal(data)

	err = os.WriteFile(filepath.Join(deckhouseRoot, "docs/documentation/_data/deckhouse-alerts.yml"), d8AlertsDataYAML, 0666)
	if err != nil {
		panic(err)
	}

	return nil
}

func getAlertsFromTemplate(templateContent []byte, moduleName, moduleUrlName, edition, sourceFile string) (alerts []moduleAlert, err error) {
	var values []map[string]interface{}

	err = yaml.Unmarshal(templateContent, &values)
	if err != nil {
		return nil, fmt.Errorf("error processing file %s - %w", sourceFile, err)
	}

	for _, value := range values {
		for _, alert := range value["rules"].([]interface{}) {
			var description, markupFormat, severity, summary string

			alertMap := alert.(map[string]interface{})

			if _, ok := alertMap["alert"]; !ok {
				// It is not an alerting rule (it is e. g. a recording rule), so skip it.
				continue
			}

			alertAnnotations, ok := alertMap["annotations"].(map[string]interface{})
			if ok {
				description, ok = alertAnnotations["description"].(string)
				if !ok {
					description = "" // or any other default value
				}
				markupFormat, ok = alertAnnotations["plk_markup_format"].(string)
				if !ok {
					markupFormat = "default"
				}
				if summary, ok = alertAnnotations["summary"].(string); !ok {
					summary = ""
				} else {
					var buf bytes.Buffer
					if err := goldmark.Convert([]byte(strings.ReplaceAll(summary, "\n", " ")), &buf); err == nil {
						summary = stripHTMLTags(string(buf.Bytes()))
						// summary = strings.TrimLeft(summary,"<p>")
						// summary = strings.TrimRight(summary,"</p>\n")
					}
				}
			}

			alertLabels, ok := alertMap["labels"].(map[string]interface{})
			severity = "undefined"
			if ok {
				// don't store severity if it is not a number (e.g. it can be a template)
				if severityData, severityExists := alertLabels["severity_level"]; severityExists {
					if _, err := strconv.Atoi(severityData.(string)); err == nil {
						severity = severityData.(string)
					}
				}
			}

			alerts = append(alerts, moduleAlert{
				Name:         alertMap["alert"].(string),
				SourceFile:   sourceFile,
				ModuleUrl:    moduleUrlName,
				Module:       moduleName,
				Edition:      edition,
				Description:  description,
				Summary:      summary,
				Severity:     severity,
				MarkupFormat: markupFormat,
			})
		}
	}

	// return yaml.Marshal(alerts)
	return alerts, nil
}

func modules(deckhouseRoot string) (modules []module) {
	// ce modules
	files, _ := os.ReadDir(filepath.Join(deckhouseRoot, "modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join("modules", file.Name()),
				Edition: ce,
			}
			modules = append(modules, md)
		}
	}

	// ee modules
	files, _ = os.ReadDir(filepath.Join(deckhouseRoot, "ee/modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join("ee/modules", file.Name()),
				Edition: ee,
			}
			modules = append(modules, md)
		}
	}

	// be modules
	files, _ = os.ReadDir(filepath.Join(deckhouseRoot, "ee/be/modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join("ee/be/modules", file.Name()),
				Edition: be,
			}
			modules = append(modules, md)
		}
	}

	// fe modules
	files, _ = os.ReadDir(filepath.Join(deckhouseRoot, "ee/fe/modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join("ee/fe/modules", file.Name()),
				Edition: fe,
			}
			modules = append(modules, md)
		}
	}

	// se modules
	files, _ = os.ReadDir(filepath.Join(deckhouseRoot, "ee/se/modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join("ee/se/modules", file.Name()),
				Edition: se,
			}
			modules = append(modules, md)
		}
	}

	files, _ = os.ReadDir(filepath.Join(deckhouseRoot, "ee/se-plus/modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join("ee/se-plus/modules", file.Name()),
				Edition: sePlus,
			}
			modules = append(modules, md)
		}
	}

	return modules
}

func moduleTemplates(module module) (yamlTemplates, tplTemplates map[string][]byte) {
	yamlTemplates = make(map[string][]byte)
	tplTemplates = make(map[string][]byte)

	readDirWithTemplates(filepath.Join(deckhouseRoot, module.Path, prometheusRules), "", yamlTemplates, tplTemplates)

	return
}

func readDirWithTemplates(pathToDir string, parentFolder string, yamlTemplates map[string][]byte, tplTemplates map[string][]byte) {
	files, err := os.ReadDir(pathToDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(pathToDir, file.Name()))
			if err != nil {
				continue
			}

			switch filepath.Ext(file.Name()) {
			case ".yaml":
				yamlTemplates[filepath.Join(parentFolder, file.Name())] = content
			case ".tpl":
				tplTemplates[filepath.Join(parentFolder, file.Name())] = content
			}
		} else {
			readDirWithTemplates(filepath.Join(pathToDir, file.Name()), file.Name(), yamlTemplates, tplTemplates)
		}
	}
	return
}

func readUserDefinedValues(moduleName string) (userDefinedValues []byte, err error) {
	userDefinedValuesPath := filepath.Join(deckhouseRoot, userDefinedValuesPath, moduleName+".yaml")

	return os.ReadFile(userDefinedValuesPath)
}

func renderHelmTemplate(module module, templateNames []string) (map[string]string, error) {
	renderDir, err := helper.NewRenderDir("renderdir")
	if err != nil {
		return nil, err
	}
	defer renderDir.Remove()

	for _, templateName := range templateNames {
		names := strings.Split(templateName, "/")
		if len(names) > 0 {
			if err := renderDir.AddTemplate(names[len(names)-1], filepath.Join(deckhouseRoot, module.Path, prometheusRules, templateName)); err != nil {
				return nil, err
			}
		} else {
			if err := renderDir.AddTemplate(templateName, filepath.Join(deckhouseRoot, module.Path, prometheusRules, templateName)); err != nil {
				return nil, err
			}
		}
	}

	renderDir.AddHelper(filepath.Join(deckhouseRoot, module.Path, helpersPath))

	userDefinedValuesRaw, err := readUserDefinedValues(module.Name)
	if err != nil {
		userDefinedValuesRaw = []byte{}
	}

	initValues, err := library.InitValues(module.Path, userDefinedValuesRaw)
	if err != nil {
		return nil, err
	}

	rawInitValues, err := json.Marshal(initValues)
	if err != nil {
		return nil, err
	}

	r := helm.Renderer{}
	result, err := r.RenderChartFromDir(renderDir.Path(), string(rawInitValues))
	if err != nil {
		templatePath := strings.ReplaceAll(err.Error(), "renderdir/templates", filepath.Join(deckhouseRoot, module.Path, prometheusRules))
		return result, fmt.Errorf("error processing template: %v", templatePath)
	}

	return result, nil
}
