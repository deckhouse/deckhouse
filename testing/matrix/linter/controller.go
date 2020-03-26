package linter

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

var (
	sep        = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
	pathRegexp = regexp.MustCompile("# Source: (.*)")
)

type ModuleController struct {
	valuesDir string
	Module    types.Module
}

func NewModuleController(tmpDir string, m types.Module) *ModuleController {
	return &ModuleController{valuesDir: tmpDir, Module: m}
}

func (c *ModuleController) GetTestCases() ([]string, int, error) {
	var files []string
	err := filepath.Walk(c.valuesDir, func(path string, info os.FileInfo, err error) error {
		if c.valuesDir == path {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, len(files), err
}

func (c *ModuleController) Run() error {
	testCases, testCasesQuantity, err := c.GetTestCases()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	waitCh := make(chan struct{})
	errorCh := make(chan error, testCasesQuantity)

	wg.Add(testCasesQuantity)
	go func() {
		for index, file := range testCases {
			go func(index int, file string) {
				defer wg.Done()

				index++

				objectStore := storage.NewUnstructuredObjectStore()
				fileContent, err := ioutil.ReadFile(file)
				if err != nil {
					errorCh <- fmt.Errorf("test #%v failed: %s", index, err)
					return
				}

				out, err := c.RunRender(file)
				if err != nil {
					errorCh <- testsError(index, err, string(fileContent), string(out))
					return
				}

				doc, err := fillObjectStore(objectStore, out)
				if err != nil {
					errorCh <- testsError(index, err, string(fileContent), doc)
					return
				}

				err = rules.ApplyLintRules(c.Module, objectStore)
				if err != nil {
					errorCh <- err
					return
				}
				objectStore.Close()
			}(index, file)
		}
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		fmt.Print(testsSuccessful(c.Module.Name, testCasesQuantity))
		return nil
	case err := <-errorCh:
		return err
	}
}

func (c *ModuleController) RunRender(values string) ([]byte, error) {
	return exec.Command("helm", "template", c.Module.Path, "--values", values).CombinedOutput() // #nosec
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

func extractManifestPath(doc string) string {
	if idx := strings.Index(doc, "\n"); idx != -1 {
		doc = doc[:idx]
	}

	matches := pathRegexp.FindStringSubmatch(doc)
	if len(matches) > 0 {
		// second capture group is a path
		return matches[1]
	}
	return ""
}

func fillObjectStore(objectStore storage.UnstructuredObjectStore, bigFile []byte) (string, error) {
	path := ""

	bigFileTmp := strings.TrimSpace(string(bigFile))
	docs := sep.Split(bigFileTmp, -1)
	for _, d := range docs {
		if d == "" {
			continue
		}

		d = strings.TrimSpace(d)

		pathCandidate := extractManifestPath(d)
		if pathCandidate != "" {
			path = pathCandidate
		}

		var node map[string]interface{}
		err := yaml.Unmarshal([]byte(d), &node)
		if err != nil {
			return d, err
		}

		if node == nil {
			continue
		}

		err = objectStore.Put(path, node)
		if err != nil {
			return d, fmt.Errorf("helm chart object already exists: %v", err)
		}
	}
	return "", nil
}
