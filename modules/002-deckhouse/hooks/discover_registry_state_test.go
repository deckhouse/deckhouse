/*
Copyright 2026 Flant JSC

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

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func registrySecret(state registryState) string {
	encodedMode := base64.StdEncoding.EncodeToString([]byte(state.Mode))

	return fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
---
apiVersion: v1
kind: Secret
metadata:
  name: registry-state
  namespace: d8-system
data:
  mode: %s`, encodedMode)
}

var _ = Describe("Deckhouse :: hooks :: discover registry state ::", func() {
	f := HookExecutionConfigInit(`
deckhouse:
  internal: {}
`, `{}`)

	Context("On begin: when no secret exists", func() {
		BeforeEach(func() {
			st := f.KubeStateSet("")
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Should set empty state in values", func() {
			Expect(f.ValuesGet(registryStateValuesPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(registryStateValuesPath).String()).To(MatchJSON(`{}`))
		})

		Context("After: when secret is created", func() {
			BeforeEach(func() {
				st := f.KubeStateSet(registrySecret(registryState{Mode: "Direct"}))
				f.BindingContexts.Set(st)
				f.RunHook()
			})

			It("Should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Should update internal values", func() {
				Expect(f.ValuesGet(registryStateValuesPath).Exists()).To(BeTrue())
				Expect(f.ValuesGet(registryStateValuesPath).String()).To(MatchJSON(`{"mode": "Direct"}`))
			})

			Context("After: when secret is changed mode", func() {
				BeforeEach(func() {
					st := f.KubeStateSet(registrySecret(registryState{Mode: "Unmanaged"}))
					f.BindingContexts.Set(st)
					f.RunHook()
				})

				It("Should execute successfully", func() {
					Expect(f).To(ExecuteSuccessfully())
				})

				It("Should update internal values", func() {
					Expect(f.ValuesGet(registryStateValuesPath).Exists()).To(BeTrue())
					Expect(f.ValuesGet(registryStateValuesPath).String()).To(MatchJSON(`{"mode": "Unmanaged"}`))
				})

				Context("After: when secret is removed", func() {
					BeforeEach(func() {
						st := f.KubeStateSet("")
						f.BindingContexts.Set(st)
						f.RunHook()
					})

					It("Should execute successfully", func() {
						Expect(f).To(ExecuteSuccessfully())
					})

					It("Should keep empty registry state", func() {
						Expect(f.ValuesGet(registryStateValuesPath).Exists()).To(BeTrue())
						Expect(f.ValuesGet(registryStateValuesPath).String()).To(MatchJSON(`{}`))
					})
				})
			})
		})
	})

	Context("On begin: when secret exists but is empty mode", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(registrySecret(registryState{Mode: ""}))
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Should set empty registry state in values", func() {
			Expect(f.ValuesGet(registryStateValuesPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(registryStateValuesPath).String()).To(MatchJSON(`{}`))
		})
	})

	Context("On begin: when secret exists with no empty mode", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(registrySecret(registryState{Mode: "Direct"}))
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Should update internal values", func() {
			Expect(f.ValuesGet(registryStateValuesPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(registryStateValuesPath).String()).To(MatchJSON(`{"mode": "Direct"}`))
		})
	})
})
