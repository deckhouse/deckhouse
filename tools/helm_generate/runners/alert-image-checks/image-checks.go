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

package alertimagechecks

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"tools/helm_generate/helper"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/iancoleman/strcase"
)

func run(templatePath, helpersPath string) error {
	renderContent, err := renderHelmTemplate(templatePath, helpersPath)
	if err != nil {
		return err
	}

	io.WriteString(os.Stdout, renderContent["renderdir/templates/template"])

	return nil
}

func renderHelmTemplate(templateName, helpersPath string) (map[string]string, error) {
	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return nil, err
	}
	renderDirPath, err := helper.NewRenderDir("renderdir")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(renderDirPath)

	templateFullPath := filepath.Join(filepath.Join(deckhouseRoot, templateName))
	if err := os.Symlink(templateFullPath, filepath.Join(renderDirPath, "/templates/template")); err != nil {
		return nil, err
	}

	if helpersPath != "" {
		helpersFullPath := filepath.Join(filepath.Join(deckhouseRoot, helpersPath))
		files, err := os.ReadDir(helpersFullPath)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if !file.IsDir() && file.Name()[0:1] == "_" {
				os.Symlink(filepath.Join(helpersFullPath, file.Name()), filepath.Join(renderDirPath, "/templates", file.Name()))
			}
		}
	}

	initValues, err := library.InitValues(getModulePathFromTemplatePath(templateFullPath), []byte{})
	if err != nil {
		return nil, err
	}

	mJson, err := json.Marshal(map[string]interface{}{
		getCamelModuleName(templateFullPath): initValues,
	})
	if err != nil {
		return nil, err
	}

	r := helm.Renderer{}
	return r.RenderChartFromDir(renderDirPath, string(mJson))
}

func getCamelModuleName(path string) string {
	var moduleCamelName string
	var inModulesDir bool

	s := strings.Split(path, "/")
	for _, v := range s {
		if inModulesDir {
			str := strings.SplitN(v, "-", 2)
			if len(str) > 1 {
				moduleCamelName = strcase.ToLowerCamel(str[1])
			}
			break
		}
		if v == "modules" {
			inModulesDir = true
		}
	}

	return moduleCamelName
}

func getModulePathFromTemplatePath(path string) string {
	var modulePath string = "/"
	var inModulesDir bool

	s := strings.Split(path, "/")
	for _, v := range s {
		modulePath = filepath.Join(modulePath, v)
		if inModulesDir {
			break
		}
		if v == "modules" {
			inModulesDir = true
		}
	}

	return modulePath
}
