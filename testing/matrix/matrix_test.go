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

package matrix

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/testing/matrix/linter"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

func TestMatrix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Matrix tests", func() {
	// use MODULES_DIR=/deckhouse/modules/000-module-name env var for run matrix tests for one module
	modules, err := modules.GetDeckhouseModulesWithValuesMatrixTests()

	modulesCH := make(chan utils.Module, len(modules))
	Context("module discovery", func() {
		It("", func() {
			Expect(err).ShouldNot(HaveOccurred())
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
