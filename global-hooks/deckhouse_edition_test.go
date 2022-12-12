// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: deckhouse_edition ", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Unknown deckhouse edition", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseEdition").String()).To(Equal(`Unknown`))
		})
	})

	Context("With set edition", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			err := writeEditionTMPFile("FE")
			Expect(err).To(BeNil())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseEdition").String()).To(Equal(`FE`))
		})
	})
})

func writeEditionTMPFile(content string) error {
	tmpfile, err := os.CreateTemp("", "deckhouse-edition-*")
	if err != nil {
		return err
	}
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return err
	}

	return os.Setenv("D8_EDITION_TMP_FILE", tmpfile.Name())
}
