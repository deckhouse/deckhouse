package modules

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
)

const (
	ginkgoImport        = `. "github.com/onsi/ginkgo"`
	gomegaImport        = `. "github.com/onsi/gomega"`
	commonTestGoContent = `
func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}
`
)

func commonTestGoForHooks(name, path string) errors.LintRuleError {
	if !isExistsOnFilesystem(path, hooksDir) {
		return errors.EmptyRuleError
	}

	if matches, _ := filepath.Glob(filepath.Join(path, hooksDir, "*.go")); len(matches) == 0 {
		return errors.EmptyRuleError
	}

	commonTestPath := filepath.Join(path, hooksDir, "common_test.go")
	if !isExistsOnFilesystem(commonTestPath) {
		return errors.NewLintRuleError(
			"MODULE001",
			moduleLabel(name),
			nil,
			"Module does not contain %q file", commonTestPath,
		)
	}

	contentBytes, err := ioutil.ReadFile(commonTestPath)
	if err != nil {
		return errors.NewLintRuleError(
			"MODULE001",
			moduleLabel(name),
			nil,
			"Module does not contain %q file", commonTestPath,
		)
	}

	var errs []string
	if !strings.Contains(string(contentBytes), commonTestGoContent) {
		errs = append(errs,
			fmt.Sprintf("Module content of %q file does not contain:\n\t%s", commonTestPath, commonTestGoContent),
		)
	}

	if !strings.Contains(string(contentBytes), gomegaImport) {
		errs = append(errs,
			fmt.Sprintf("Module content of %q file does not contain:\n\t%s", commonTestPath, gomegaImport),
		)
	}

	if !strings.Contains(string(contentBytes), ginkgoImport) {
		errs = append(errs,
			fmt.Sprintf("Module content of %q file does not contain:\n\t%s", commonTestPath, ginkgoImport),
		)
	}

	if len(errs) > 0 {
		errstr := strings.Join(errs, "\n")

		return errors.NewLintRuleError(
			"MODULE001",
			moduleLabel(name),
			nil,
			errstr,
		)
	}

	return errors.EmptyRuleError
}
