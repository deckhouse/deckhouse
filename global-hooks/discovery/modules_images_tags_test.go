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

var _ = Describe("Global hooks :: discovery :: modules_images_tags ", func() {
	f := HookExecutionConfigInit(`{"global": {"modulesImages": {}}}`, `{}`)

	Context("Tags files not exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should return error", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("Tags files exists", func() {
		Context("Valid json object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateOnStartupContext())

				const content = `{"basicAuth": {"nginx": "valid-tag"}}`
				err := writeTagsTMPFile(content)
				Expect(err).To(BeNil())
				f.RunHook()
			})

			It("Should set tags files content as object into 'global.modulesImages.tags'", func() {
				Expect(f).To(ExecuteSuccessfully())
				tag := f.ValuesGet("global.modulesImages.tags").String()
				Expect(tag).To(MatchJSON(`
{
	"basicAuth": {
	  "nginx": "valid-tag"
	},
	"testLocal": {
	  "test": "valid-tag"
	},
	"testTest": {
	  "test": "valid-tag"
	}
}`))
			})
		})

		Context("Valid json string", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateOnStartupContext())

				const content = `"basicAuth"`
				err := writeTagsTMPFile(content)
				Expect(err).To(BeNil())
				f.RunHook()
			})

			It("Should return error", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
			})
		})

		Context("Valid json array", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateOnStartupContext())

				const content = `["basicAuth"]`
				err := writeTagsTMPFile(content)
				Expect(err).To(BeNil())
				f.RunHook()
			})

			It("Should return error", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
			})
		})

		Context("invalid json", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateOnStartupContext())

				const content = `{"basicAuth": {"nginx": "valid-tag"}`
				err := writeTagsTMPFile(content)
				Expect(err).To(BeNil())
				f.RunHook()
			})

			It("Should return error", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
			})
		})

	})
})

func writeTagsTMPFile(content string) error {
	tmpfile, err := os.CreateTemp("", "d8-modules-images-tags-*")
	if err != nil {
		return err
	}
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return err
	}

	return os.Setenv("D8_TAGS_TMP_FILE", tmpfile.Name())
}
