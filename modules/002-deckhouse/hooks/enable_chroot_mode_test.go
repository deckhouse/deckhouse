/*
Copyright 2025 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("deckhouse :: hooks :: chroot mode ::", func() {
	f := HookExecutionConfigInit(`
deckhouse:
  internal: {}
`, `{}`)

	Context("No configmap - hooks executes correctly", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(``)
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Shouldn't change internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.chrootMode").Bool()).To(BeFalse())
		})
	})

	Context("Configmap chroot-mode is in place", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: chroot-mode
  namespace: d8-system
`)
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should update internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.chrootMode").Bool()).To(BeTrue())
		})
	})
})
