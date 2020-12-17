package linter

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/flant/addon-operator/pkg/module_manager"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
	"github.com/iancoleman/strcase"
	"github.com/kyokomi/emoji"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
	"github.com/deckhouse/deckhouse/testing/util/helm"
)

func init() {
	// Mute Shell-Operator logs
	logrus.SetLevel(logrus.PanicLevel)
}

var (
	workersQuantity = runtime.NumCPU()

	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func loadOpenAPISchemas(m *types.Module) error {
	openAPIDir := filepath.Join("deckhouse", "global", "openapi")
	configBytes, valuesBytes, err := module_manager.ReadOpenAPISchemas(openAPIDir)
	if err != nil {
		return fmt.Errorf("read global openAPI schemas: %v", err)
	}
	if configBytes != nil {
		err = validation.AddGlobalValuesSchema("config", configBytes)
		if err != nil {
			return fmt.Errorf("parse global config openAPI: %v", err)
		}
	}
	if valuesBytes != nil {
		err = validation.AddGlobalValuesSchema("memory", valuesBytes)
		if err != nil {
			return fmt.Errorf("parse global values openAPI: %v", err)
		}
	}

	openAPIPath := filepath.Join(m.Path, "openapi")
	valuesKey := strcase.ToLowerCamel(m.Name)
	configBytes, valuesBytes, err = module_manager.ReadOpenAPISchemas(openAPIPath)
	if err != nil {
		return fmt.Errorf("module '%s' read openAPI schemas: %v", m.Name, err)
	}
	if configBytes != nil {
		err = validation.AddModuleValuesSchema(valuesKey, "config", configBytes)
		if err != nil {
			return fmt.Errorf("module '%s' parse config openAPI: %v", m.Name, err)
		}
	}
	if valuesBytes != nil {
		err = validation.AddModuleValuesSchema(valuesKey, "memory", valuesBytes)
		if err != nil {
			return fmt.Errorf("module '%s' parse config openAPI: %v", m.Name, err)
		}
	}

	return nil
}

type ModuleController struct {
	Module types.Module
	Values []string
	Chart  *chart.Chart
}

func NewModuleController(m types.Module, values []string) *ModuleController {
	// Check chart requirements to make sure all dependencies are present in /charts
	hc, err := loader.Load(m.Path)
	if err != nil {
		panic(fmt.Errorf("chart load: %v", err))
	}

	if err := loadOpenAPISchemas(&m); err != nil {
		panic(fmt.Errorf("schemas load: %v", err))
	}

	return &ModuleController{Module: m, Values: values, Chart: hc}
}

type Task struct {
	index  int
	values string
}

type Worker struct {
	id       int
	tasksCh  chan Task
	errorsCh chan error
	doneCh   chan struct{}

	ctx context.Context
}

func NewWorker(ctx context.Context, id int, tasksCh chan Task, errorsCh chan error, doneCh chan struct{}) *Worker {
	return &Worker{id: id, tasksCh: tasksCh, errorsCh: errorsCh, doneCh: doneCh, ctx: ctx}
}

func (w *Worker) Start(c *ModuleController) {
	for {
		select {
		case task := <-w.tasksCh:
			err := c.ValidateValues(task.values)
			if err != nil {
				w.errorsCh <- testsError(task.index, err, task.values)
				return
			}

			objectStore := storage.NewUnstructuredObjectStore()

			err = c.RunRender(task.values, &objectStore)
			if err != nil {
				w.errorsCh <- testsError(task.index, err, task.values)
				return
			}

			err = rules.ApplyLintRules(c.Module, task.values, &objectStore)
			if err != nil {
				w.errorsCh <- err
				return
			}
			w.doneCh <- struct{}{}
		case <-w.ctx.Done():
			return
		}
	}
}

func (c *ModuleController) Run() error {
	testCasesQuantity := len(c.Values)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	errorsCh := make(chan error, testCasesQuantity)
	tasksCh := make(chan Task)
	doneCh := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for id := 0; id <= workersQuantity; id++ {
		go NewWorker(ctx, id, tasksCh, errorsCh, doneCh).Start(c)
	}

	go func() {
		for index, valuesData := range c.Values {
			tasksCh <- Task{index: index, values: valuesData}
		}
	}()

	doneCounter := 0
	for {
		select {
		case <-doneCh:
			doneCounter++
			if doneCounter == testCasesQuantity {
				fmt.Print(testsSuccessful(c.Module.Name, testCasesQuantity))
				return nil
			}
		case s := <-signalCh:
			fmt.Printf("\nReceived signal %s, exiting...\n", s)
			return nil
		case err := <-errorsCh:
			return err
		}
	}
}

func (c *ModuleController) RunRender(values string, objectStore *storage.UnstructuredObjectStore) error {
	var renderer helm.Renderer
	renderer.Name = c.Module.Name
	renderer.Namespace = c.Module.Namespace
	renderer.LintMode = true
	files, err := renderer.RenderChart(c.Chart, values)
	if err != nil {
		return fmt.Errorf("helm chart render: %v", err)
	}

	for path, bigFile := range files {
		bigFileTmp := strings.TrimSpace(bigFile)
		docs := sep.Split(bigFileTmp, -1)
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
	return nil
}

func (c *ModuleController) ValidateValues(values string) error {
	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(values), &obj)
	if err != nil {
		return err
	}

	valuesKey := strcase.ToLowerCamel(c.Module.Name)
	schemaValidation := []struct {
		schema *spec.Schema
		key    string
	}{
		{
			schema: validation.GetGlobalValuesSchema("memory"),
			key:    "global",
		},
		{
			schema: validation.GetGlobalValuesSchema("config"),
			key:    "global",
		},
		{
			schema: validation.GetModuleValuesSchema(c.Module.Name, "memory"),
			key:    valuesKey,
		},
		{
			schema: func() *spec.Schema {
				s := validation.GetModuleValuesSchema(c.Module.Name, "config")
				if s == nil {
					return s
				}
				// Do not validate internal values with config schema
				s.Properties["internal"] = spec.Schema{}
				return s
			}(),
			key: valuesKey,
		},
	}

	for _, ss := range schemaValidation {
		if ss.schema == nil {
			continue
		}

		err = validation.ValidateObject(obj[ss.key], ss.schema, ss.key)
		if err != nil {
			return err
		}
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
%s`
	testsSuccessfulMessage = `
%sModule %s - %v test cases passed!`
	testsErrorMessage = `test #%v failed:
--- Error:
%s

--- Values:
%s
`
)
