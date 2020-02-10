package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/flant/shell-operator/pkg/utils/manifest/releaseutil"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

type GVR struct {
	Kind      string
	Name      string
	Namespace string
}

type UnstructuredObjectStore map[GVR]unstructured.Unstructured

func (store UnstructuredObjectStore) PutObjectSafely(object unstructured.Unstructured, index GVR) error {
	if _, ok := store[index]; ok == true {
		return fmt.Errorf("object %s already exists in the object store", index)
	}
	store[index] = object
	return nil
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
			objectStore := UnstructuredObjectStore{}
			fileContent, _ := ioutil.ReadFile(file)

			defer wg.Done()
			helmCmd := exec.Command("helm", "template", c.ModuleDir, "--values", file)
			out, err := helmCmd.CombinedOutput()
			if err != nil {
				errorCh <- fmt.Errorf("test #%v failed: %v\n\n----- # %s\n%s\n-----\n%v", index, err, file, string(fileContent), string(out))
				return
			}
			for _, doc := range releaseutil.SplitManifests(string(out)) {
				var t interface{}

				err = yaml.Unmarshal([]byte(doc), &t)
				if err != nil {
					errorCh <- fmt.Errorf("test #%v failed: %v\n\n----- # %s\n%s\n-----\n%v", index, err, file, string(fileContent), doc)
					return
				}
				if t == nil {
					continue
				}

				var unstructuredObj unstructured.Unstructured
				unstructuredObj.SetUnstructuredContent(t.(map[string]interface{}))

				err = objectStore.PutObjectSafely(unstructuredObj, GVR{
					Kind:      unstructuredObj.GetKind(),
					Namespace: unstructuredObj.GetNamespace(),
					Name:      unstructuredObj.GetName(),
				})
				if err != nil {
					errorCh <- fmt.Errorf("test #%v failed: %v\n\n----- # %s\n%s\n-----\n%v", index, fmt.Errorf("helm output already has object: %v", err), file, string(fileContent), string(out))
					return
				}
				err = ApplyLintRules(objectStore)
				if err != nil {
					errorCh <- fmt.Errorf("test #%v failed: %v\n\n----- # %s\n%s\n-----\n%v", index, fmt.Errorf("lint rule failed: %v", err), file, string(fileContent), string(out))
					return
				}
			}
		}(index, file)
	}
	wg.Wait()
	if len(errorCh) > 0 {
		return fmt.Errorf("%v of %v values tests failed\n\n%v", len(errorCh), len(files), <-errorCh)
	}
	return nil
}
