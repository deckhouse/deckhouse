package matrix

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/testing/matrix/linter"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

func TestMatrix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Matrix tests", func() {
	modules, err := modules.GetDeckhouseModulesWithValuesMatrixTests()

	modulesCH := make(chan types.Module, len(modules))
	Context("module discovery", func() {
		It("", func() {
			Expect(err).ToNot(ErrorOccurred())
		})
	})

	Context("run", func() {
		for _, module := range modules {
			modulesCH <- module
			It("for module "+module.Name, func() {
				Expect(linter.Run("", <-modulesCH)).ToNot(ErrorOccurred())
			})
		}
	})
})
