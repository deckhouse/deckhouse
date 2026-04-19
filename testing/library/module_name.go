/*
Copyright 2021 Flant JSC

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

package library

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"sigs.k8s.io/yaml"
)

var moduleDirNameRegex = regexp.MustCompile(`^(\d+-)(.+)$`)

func GetModuleNameByPath(modulePath string) (string, error) {
	chartYamlBytes, err := os.ReadFile(modulePath + "/Chart.yaml")
	if err == nil {
		return extractModuleNameFromChartYaml(chartYamlBytes)
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read file: %w", err)
	}

	dirName := filepath.Base(modulePath)

	res := moduleDirNameRegex.FindStringSubmatch(dirName)
	if len(res) < 2 {
		return "", fmt.Errorf(`cannot get moduleName from dir %q. Regex ""^(\d+-)(.+)$" not matching second group`, dirName)
	}
	return res[2], nil
}

func extractModuleNameFromChartYaml(chartYamlBytes []byte) (string, error) {
	var chart struct {
		Name string `yaml:"name"`
	}

	err := yaml.Unmarshal(chartYamlBytes, &chart)
	if err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	if len(chart.Name) == 0 {
		return "", fmt.Errorf(`"Chart.yaml"'s Name field is empty`)
	}

	return chart.Name, nil
}
