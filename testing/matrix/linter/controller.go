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
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/hashstructure/v2"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/deckhouse/deckhouse/testing/library/values_validation"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

var (
	workersQuantity = runtime.NumCPU() * 32

	renderedTemplatesHash = sync.Map{}
)

type ModuleController struct {
	Module         utils.Module
	Values         []chartutil.Values
	valueValidator *values_validation.ValuesValidator
}

func NewModuleController(m utils.Module, values []chartutil.Values) *ModuleController {
	valueValidator, err := values_validation.NewValuesValidator(m.Name, m.Path)
	if err != nil {
		panic(fmt.Errorf("schemas load: %v", err))
	}

	return &ModuleController{
		Module:         m,
		Values:         values,
		valueValidator: valueValidator,
	}
}

type Task struct {
	index  int
	values chartutil.Values
}

func NewTask(index int, values chartutil.Values) *Task {
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

func (c *ModuleController) RunRender(values chartutil.Values, objectStore *storage.UnstructuredObjectStore) (lintError error) {
	var renderer helm.Renderer
	renderer.Name = c.Module.Name
	renderer.Namespace = c.Module.Namespace
	renderer.LintMode = true

	files, err := renderer.RenderChartFromRawValues(c.Module.Chart, values)
	if err != nil {
		lintError = fmt.Errorf("helm chart render: %v", err)
		return
	}

	hash, err := hashstructure.Hash(files, hashstructure.FormatV2, nil)
	if err != nil {
		lintError = fmt.Errorf("helm chart render: %v", err)
		return
	}

	if _, ok := renderedTemplatesHash.Load(hash); ok {
		return // the same files were already checked
	}

	defer renderedTemplatesHash.Store(hash, struct{}{})

	var docBytes []byte

	for path, bigFile := range files {
		scanner := bufio.NewScanner(strings.NewReader(bigFile))
		scanner.Split(SplitAt("---"))

		for scanner.Scan() {
			var node map[string]interface{}
			docBytes = scanner.Bytes()

			err := yaml.Unmarshal(docBytes, &node)
			if err != nil {
				return fmt.Errorf(manifestErrorMessage, err, numerateManifestLines(string(docBytes)))
			}

			if len(node) == 0 {
				continue
			}

			err = objectStore.Put(path, node, docBytes)
			if err != nil {
				return fmt.Errorf("helm chart object already exists: %v", err)
			}
		}
	}
	return
}

func lint(c *ModuleController, task *Task) error {
	err := c.valueValidator.ValidateValues(c.Module.Name, task.values)
	if err != nil {
		return testsError(task.index, err, task.values)
	}

	objectStore := storage.NewUnstructuredObjectStore()
	err = c.RunRender(task.values, objectStore)
	if err != nil {
		return testsError(task.index, err, task.values)
	}

	err = ApplyLintRules(c.Module, task.values, objectStore)
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

func testsError(index int, errorHeader error, generatedValues chartutil.Values) error {
	data, err := yaml.Marshal(generatedValues)
	if err != nil {
		panic(err.Error()) // generated values are always valid YAML-formatted documents
	}
	return fmt.Errorf(testsErrorMessage, index, errorHeader, data)
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

func SplitAt(substring string) func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Return nothing if at end of file and no data passed
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Find the index of the input of the separator substring
		if i := strings.Index(string(data), substring); i >= 0 {
			return i + len(substring), data[0:i], nil
		}

		// If at end of file with data return the data
		if atEOF {
			return len(data), data, nil
		}

		return
	}
}
