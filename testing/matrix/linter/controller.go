package linter

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/helm/helm/pkg/renderutil"
	"github.com/helm/helm/pkg/timeconv"
	"github.com/kyokomi/emoji"
	"gopkg.in/yaml.v3"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

var (
	workersQuantity = runtime.NumCPU()

	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

type ModuleController struct {
	Module types.Module
	Values []string
	Chart  *chart.Chart
}

func NewModuleController(m types.Module, values []string) *ModuleController {
	// Check chart requirements to make sure all dependencies are present in /charts
	hc, err := chartutil.Load(m.Path)
	if err != nil {
		panic(fmt.Errorf("chart load: %v", err))
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
			objectStore := storage.NewUnstructuredObjectStore()

			err := c.RunRender(task.values, &objectStore)
			if err != nil {
				w.errorsCh <- testsError(task.index, err, task.values, "")
				return
			}

			err = rules.ApplyLintRules(c.Module, objectStore)
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
	data, err := renderutil.Render(c.Chart,
		&chart.Config{Raw: values, Values: map[string]*chart.Value{}},
		renderutil.Options{
			ReleaseOptions: chartutil.ReleaseOptions{
				Name:      c.Module.Name,
				IsInstall: true,
				IsUpgrade: true,
				Time:      timeconv.Now(),
				Namespace: c.Module.Namespace,
			},
		})
	if err != nil {
		return fmt.Errorf("chart render: %v", err)
	}

	for path, bigFile := range data {
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
				return fmt.Errorf("manifest unmarshal: %v\n--- Manifest:%s", err, d)
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

func testsSuccessful(moduleName string, testCasesQuantity int) string {
	return fmt.Sprintf("\n%sModule %s - %v test cases passed! ",
		emoji.Sprint(":see_no_evil:"),
		color.New(color.FgBlue).SprintFunc()("["+moduleName+"]"),
		testCasesQuantity,
	)
}

func testsError(index int, errorHeader error, generatedValues, doc string) error {
	return fmt.Errorf("test #%v failed: %s\n\n-----\n%s\n\n-----\n%s", index, errorHeader, generatedValues, doc)
}
