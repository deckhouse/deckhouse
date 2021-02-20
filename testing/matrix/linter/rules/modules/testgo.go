package modules

import (
	"io/ioutil"
	"path/filepath"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
)

const commonTestGoContent = `package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}
`

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

	if string(contentBytes) != commonTestGoContent {
		return errors.NewLintRuleError(
			"MODULE001",
			moduleLabel(name),
			nil,
			"Module content of %q file is different from default\nContent should be equal to:\n%s",
			commonTestPath, commonTestGoContent,
		)
	}

	return errors.EmptyRuleError
}
