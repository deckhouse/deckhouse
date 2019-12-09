package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/testing/matrix/runner"
)

var defaultDeckhouseModulesDir = "/deckhouse/modules"

func getDeckhouseModulesWithValuesMatrixTests() map[string]string {
	modules := make(map[string]string)

	modulesDir, ok := os.LookupEnv("MODULES_DIR")
	if !ok {
		modulesDir = defaultDeckhouseModulesDir
	}

	_ = filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		_, err = os.Stat(path + "/" + runner.ValuesConfigFilename)
		if err == nil {
			parts := strings.Split(path, "/")
			name := parts[len(parts)-1]
			modules[name] = path
		}
		return nil
	})
	return modules
}

func TestMatrix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Matrix tests", func() {
	modules := getDeckhouseModulesWithValuesMatrixTests()
	modulesCH := make(chan string, len(modules))

	Context("for module", func() {
		for name, path := range modules {
			modulesCH <- path
			It(name, func() {
				err := runner.RunLint(<-modulesCH, "")
				Expect(err).ToNot(HaveOccurred())
			})
		}
	})
})
