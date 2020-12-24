package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"sigs.k8s.io/yaml"
)

var moduleDirNameRegex = regexp.MustCompile(`^(\d+-)(.+)$`)

func GetModuleNameByPath(modulePath string) (string, error) {
	chartYamlBytes, err := ioutil.ReadFile(modulePath + "/Chart.yaml")
	if err == nil {
		return extractModuleNameFromChartYaml(chartYamlBytes)
	}

	if !os.IsNotExist(err) {
		return "", err
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
		return "", err
	}

	if len(chart.Name) == 0 {
		return "", fmt.Errorf(`"Chart.yaml"'s Name field is empty`)
	}

	return chart.Name, err
}
