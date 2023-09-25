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
	"tools/helm_generate/helper"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"gopkg.in/yaml.v3"
)

// ../deckhause/tools/helm_generate/runners/alert_templates/template_values/[module-name].yaml
const userDefinedValuesPath = "tools/helm_generate/runners/alert_templates/template_values"

// ../deckhause/modules/[module-name]/monitoring/prometheus-rules[template-name].yaml or .tpl
const prometheusRules = "monitoring/prometheus-rules"

// ../deckhause/modules/[module-name]/templates/_[helper-name].yaml or .tpl
const helpersPath = "templates"

// Deckhouse edition type
const (
	ce edition = "ce" // community edition
	ee edition = "ee" // enterprise edition
	fe edition = "fe" // fan edition
)

type edition string

type module struct {
	Name    string
	Path    string
	Edition edition
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
				templateContent, err = injectToYaml(templateContent, module.Name, string(module.Edition), filepath.Join(module.Path, templateName))
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
				templateContent, err := injectToYaml([]byte(templateContent), module.Name, string(module.Edition), filepath.Join(module.Path, templateName))
				if err != nil {
					return err
				}

				io.WriteString(os.Stdout, string(templateContent))
			}
		}
	}

	return nil
}

func injectToYaml(templateContent []byte, name, edition, sourceFile string) ([]byte, error) {
	var values []map[string]interface{}
	err := yaml.Unmarshal(templateContent, &values)
	if err != nil {
		return nil, fmt.Errorf("error processing file %s - %w", sourceFile, err)
	}
	values[0]["module"] = name
	values[0]["edition"] = edition
	values[0]["sourceFile"] = sourceFile

	return yaml.Marshal(values)
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
	return r.RenderChartFromDir(renderDir.Path(), string(rawInitValues))
}
