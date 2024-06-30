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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"tools/helm_generate/helper"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/helm"
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
	ce edition = "ce" // community edition
	ee edition = "ee" // enterprise edition
	be edition = "be" // basic edition
	se edition = "se" // standard edition
	fe edition = "fe" // fan edition
)

type edition string

type module struct {
	Name    string
	Path    string
	Edition edition
}

type moduleAlert struct {
	Name         string `yaml:"name"`
	SourceFile   string `yaml:"sourceFile"`
	Module       string `yaml:"module"`
	Edition      string `yaml:"edition"`
	Description  string `yaml:"description"`
	Summary      string `yaml:"summary"`
	Severity     string `yaml:"severity"`
	MarkupFormat string `yaml:"markupFormat"`
}

func run() error {
	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return err
	}

	for _, module := range modules(deckhouseRoot) {
		yamlTemplates, tplTemplates := moduleTemplates(module)

		if len(yamlTemplates) > 0 {
			for templateName, templateContent := range yamlTemplates {
				templateContent, err = buildYaml(templateContent, module.Name, string(module.Edition), filepath.Join(module.Path, prometheusRules, templateName))
				if err != nil {
					return err
				}

				io.WriteString(os.Stdout, string(templateContent))
			}
		}

		if len(tplTemplates) > 0 {
			renderContent, err := renderHelmTemplate(module, helper.GetMapKeys(tplTemplates))
			if err != nil {
				return err
			}
			for templatePath, templateContent := range renderContent {
				_, templateName := filepath.Split(templatePath)
				templateContent, err := buildYaml([]byte(templateContent), module.Name, string(module.Edition), filepath.Join(module.Path, prometheusRules, templateName))
				if err != nil {
					return err
				}

				io.WriteString(os.Stdout, string(templateContent))
			}
		}
	}

	return nil
}

func buildYaml(templateContent []byte, name, edition, sourceFile string) ([]byte, error) {
	var values []map[string]interface{}
	var alerts []moduleAlert

	err := yaml.Unmarshal(templateContent, &values)
	if err != nil {
		return nil, fmt.Errorf("error processing file %s - %w", sourceFile, err)
	}

	if substr := strings.SplitN(name, "-", 2); len(substr) > 1 {
		name = substr[1]
	}

	for _, value := range values {
		for _, alert := range value["rules"].([]interface{}) {
			var description, markupFormat, severity, summary string

			alertMap := alert.(map[string]interface{})

			if _, ok := alertMap["alert"]; !ok {
				continue // it is not an alerting rule (it is e. g. a recording rule), skip it
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
				summary, ok = alertAnnotations["summary"].(string)
				if !ok {
					summary = ""
				}
			}

			alertLabels, ok := alertMap["labels"].(map[string]interface{})
			if ok {
				severity, ok = alertLabels["severity_level"].(string)
				if !ok {
					severity = "undefined"
				}
			}

			alerts = append(alerts, moduleAlert{
				Name:         alertMap["alert"].(string),
				SourceFile:   sourceFile,
				Module:       name,
				Edition:      edition,
				Description:  description,
				Summary:      summary,
				Severity:     severity,
				MarkupFormat: markupFormat,
			})
		}
	}

	return yaml.Marshal(alerts)
}

func modules(deckhouseRoot string) (modules []module) {
	// ce modules
	files, _ := os.ReadDir(filepath.Join(deckhouseRoot, "modules"))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name:    file.Name(),
				Path:    filepath.Join(deckhouseRoot, "modules", file.Name()),
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
				Path:    filepath.Join(deckhouseRoot, "ee/modules", file.Name()),
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
				Path:    filepath.Join(deckhouseRoot, "ee/be/modules", file.Name()),
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
				Path:    filepath.Join(deckhouseRoot, "ee/fe/modules", file.Name()),
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
				Path:    filepath.Join(deckhouseRoot, "ee/se/modules", file.Name()),
				Edition: se,
			}
			modules = append(modules, md)
		}
	}

	return modules
}

func moduleTemplates(module module) (yamlTemplates, tplTemplates map[string][]byte) {
	yamlTemplates = make(map[string][]byte)
	tplTemplates = make(map[string][]byte)

	files, err := os.ReadDir(filepath.Join(module.Path, prometheusRules))
	if err != nil {
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(module.Path, prometheusRules, file.Name()))
			if err != nil {
				continue
			}

			switch filepath.Ext(file.Name()) {
			case ".yaml":
				yamlTemplates[file.Name()] = content
			case ".tpl":
				tplTemplates[file.Name()] = content
			}
		}
	}

	return
}

func readUserDefinedValues(moduleName string) (userDefinedValues []byte, err error) {
	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return nil, err
	}

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
		if err := renderDir.AddTemplate(templateName, filepath.Join(module.Path, prometheusRules, templateName)); err != nil {
			return nil, err
		}
	}

	renderDir.AddHelper(filepath.Join(module.Path, helpersPath))

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
		templatePath := strings.ReplaceAll(err.Error(), "renderdir/templates", filepath.Join(module.Path, prometheusRules))
		return result, fmt.Errorf("error processing template: %v", templatePath)
	}

	return result, nil
}
