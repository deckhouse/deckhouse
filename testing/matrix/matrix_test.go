package matrix

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
)

func TestMatrix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Matrix tests", func() {
	_, err := modules.GetDeckhouseModulesWithValuesMatrixTests()

	Context("module discovery", func() {
		It("", func() {
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
