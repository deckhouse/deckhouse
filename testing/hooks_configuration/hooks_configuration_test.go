//go:build validation
// +build validation

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

package hooks_configuration

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestValidationHooksConfiguration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Hooks configuration tests", func() {
	hooks, err := GetAllHooks()
	Context("hooks discovery", func() {
		It("", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(hooks).ToNot(HaveLen(0))
		})
	})

	hooksCH := make(chan Hook, len(hooks))
	Context("run", func() {
		for _, hook := range hooks {
			hooksCH <- hook
			It(hook.Path, func() {
				ithook := <-hooksCH

				By("Hook file should be executable", func() {
					Expect(ithook.Executable).To(BeTrue())
				})

				err := ithook.ExecuteGetConfig()
				By(ithook.Path+" --config must not fail", func() {
					Expect(err).ToNot(HaveOccurred())

				})

				By("keepFullObjectsInMemory is mandatory for kubernetes entries", func() {
					if ithook.HookConfig.Get("kubernetes").Exists() {
						kubernetesEntries := ithook.HookConfig.Get("kubernetes").Array()
						for _, value := range kubernetesEntries {
							Expect(value.Get("keepFullObjectsInMemory").Exists()).To(BeTrue())
						}
					}
				})
			})
		}
	})
})
