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

package linter

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/kyokomi/emoji"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/deckhouse/deckhouse/testing/library/values_validation"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

var (
	workersQuantity = runtime.NumCPU() * 8
)

type ModuleController struct {
	Module          utils.Module
	Values          []string
	Chart           *chart.Chart
	ValuesValidator *validation.ValuesValidator
}

func NewModuleController(m utils.Module, values []string) *ModuleController {
	// Check chart requirements to make sure all dependencies are present in /charts
	hc, err := loader.Load(m.Path)
	if err != nil {
		panic(fmt.Errorf("chart load: %v", err))
	}

	validator := validation.NewValuesValidator()
	if err := values_validation.LoadOpenAPISchemas(validator, m.Name, m.Path); err != nil {
		panic(fmt.Errorf("schemas load: %v", err))
	}

	return &ModuleController{
		Module:          m,
		Values:          values,
		Chart:           hc,
		ValuesValidator: validator,
	}
}

type Task struct {
	index  int
	values string
}

func NewTask(index int, values string) *Task {
	return &Task{index: index, values: values}
}

type Worker struct {
	id       int
	tasksCh  <-chan *Task
	errorsCh chan<- error
}

func NewWorker(id int, tasksCh <-chan *Task, errorsCh chan<- error) *Worker {
	return &Worker{id: id, tasksCh: tasksCh, errorsCh: errorsCh}
}

func (w *Worker) Start(wg *sync.WaitGroup, c *ModuleController) {
	defer wg.Done()
	for task := range w.tasksCh {
		if err := lint(c, task); err != nil {
			w.errorsCh <- err
			return
		}
	}
}

func (c *ModuleController) Run() error {
	testCasesQuantity := len(c.Values)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	errorsCh := make(chan error, testCasesQuantity)
	tasksCh := make(chan *Task)
	doneCh := make(chan struct{})

	var wg sync.WaitGroup
	for id := 0; id <= workersQuantity; id++ {
		wg.Add(1)
		go NewWorker(id, tasksCh, errorsCh).Start(&wg, c)
	}

	go func() {
		for index, valuesData := range c.Values {
			tasksCh <- NewTask(index, valuesData)
		}

		close(tasksCh)
		wg.Wait()
		close(doneCh)
	}()

	for {
		select {
		case <-doneCh:
			fmt.Print(testsSuccessful(c.Module.Name, testCasesQuantity))
			return nil
		case s := <-signalCh:
			fmt.Printf("\nReceived signal %s, exiting...\n", s)
			return nil
		case err := <-errorsCh:
			return err
		}
	}
}

func (c *ModuleController) RunRender(values string, objectStore *storage.UnstructuredObjectStore) (lintError error) {
	var renderer helm.Renderer
	renderer.Name = c.Module.Name
	renderer.Namespace = c.Module.Namespace
	renderer.LintMode = true

	files, err := renderer.RenderChart(c.Chart, values)
	if err != nil {
		lintError = fmt.Errorf("helm chart render: %v", err)
		return
	}
	for path, bigFile := range files {
		bigFileTmp := strings.TrimSpace(bigFile)

		// Naive implementation to avoid using regex here
		docs := strings.Split(bigFileTmp, "---")
		for _, d := range docs {
			if d == "" {
				continue
			}
			d = strings.TrimSpace(d)

			var node map[string]interface{}
			err := yaml.Unmarshal([]byte(d), &node)
			if err != nil {
				return fmt.Errorf(manifestErrorMessage, err, numerateManifestLines(d))
			}

			if node == nil {
				continue
			}

			err = objectStore.Put(path, node)
			if err != nil {
				return fmt.Errorf("helm chart object already exists: %v", err)
			}
		}
	}
	return
}

func lint(c *ModuleController, task *Task) error {
	err := values_validation.ValidateValues(c.ValuesValidator, c.Module.Name, task.values)
	if err != nil {
		return testsError(task.index, err, task.values)
	}

	objectStore := storage.NewUnstructuredObjectStore()

	err = c.RunRender(task.values, &objectStore)
	if err != nil {
		return testsError(task.index, err, task.values)
	}

	err = ApplyLintRules(c.Module, task.values, &objectStore)
	if err != nil {
		return err

	}
	return nil
}

func testsSuccessful(moduleName string, testCasesQuantity int) string {
	return fmt.Sprintf(
		testsSuccessfulMessage,
		emoji.Sprint(":see_no_evil:"),
		color.New(color.FgBlue).SprintFunc()("["+moduleName+"]"),
		testCasesQuantity,
	)
}

func testsError(index int, errorHeader error, generatedValues string) error {
	return fmt.Errorf(testsErrorMessage, index, errorHeader, generatedValues)
}

func numerateManifestLines(manifest string) string {
	manifestLines := strings.Split(manifest, "\n")
	builder := strings.Builder{}

	for index, line := range manifestLines {
		builder.WriteString(fmt.Sprintf("%d\t%s\n", index+1, line))
	}

	return builder.String()
}

const (
	manifestErrorMessage = `manifest unmarshal: %v

--- Manifest:
%s
`
	testsSuccessfulMessage = `
%sModule %s - %v test cases passed!

`
	testsErrorMessage = `test #%v failed:
--- Error:
%s

--- Values:
%s

`
)
