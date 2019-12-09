package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

var (
	ChartFilename        = "Chart.yaml"
	ValuesConfigFilename = "values_matrix_test.yaml"
)

type Chart struct {
	Name string `yaml:"name"`
}

type ModuleController struct {
	ValuesDir string
	ModuleDir string
	ChartName string
}

func NewModuleController(valuesDir, moduleDir string) (ModuleController, error) {
	moduleDir = strings.TrimSuffix(moduleDir, "/")
	valuesDir = strings.TrimSuffix(valuesDir, "/")

	var chart Chart
	chartFile, err := ioutil.ReadFile(moduleDir + "/" + ChartFilename)
	if err != nil {
		return ModuleController{}, err
	}
	err = yaml.Unmarshal(chartFile, &chart)
	if err != nil {
		return ModuleController{}, err
	}

	return ModuleController{ValuesDir: valuesDir, ModuleDir: moduleDir, ChartName: chart.Name}, nil
}

func (c *ModuleController) Run() error {
	var files []string
	err := filepath.Walk(c.ValuesDir, func(path string, info os.FileInfo, err error) error {
		if c.ValuesDir == path {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	errorCh := make(chan error, len(files))
	for index, file := range files {
		wg.Add(1)
		go func(index int, file string) {
			defer wg.Done()
			helmCmd := exec.Command("helm", "template", c.ModuleDir, "--values", file)
			out, err := helmCmd.CombinedOutput()
			if err != nil {
				fileContent, _ := ioutil.ReadFile(file)
				errorCh <- fmt.Errorf("test #%v failed: %v\n\n----- # %s\n%s\n-----\n%v", index, err, file, string(fileContent), string(out))
			}
		}(index, file)
	}
	wg.Wait()
	if len(errorCh) > 0 {
		return fmt.Errorf("%v of %v values tests failed\n\n%v", len(errorCh), len(files), <-errorCh)
	}
	return nil
}
