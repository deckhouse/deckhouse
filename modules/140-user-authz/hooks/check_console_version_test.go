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

var _ = Describe("user-authz :: hooks :: check_console_version ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Module", false)

	Context("Console module is not installed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should set consoleLegacyCompat to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeFalse())
			Expect(f.ValuesGet("userAuthz.internal.consoleVersion").Exists()).To(BeFalse())
		})
	})

	Context("Console module is installed with old version (v1.43.2)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties:
  version: v1.43.2
status:
  phase: Ready
`))
			f.RunHook()
		})

		It("Should set consoleLegacyCompat to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeTrue())
			Expect(f.ValuesGet("userAuthz.internal.consoleVersion").String()).To(Equal("v1.43.2"))
		})
	})

	Context("Console module is installed with threshold version (v1.44.0)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties:
  version: v1.44.0
status:
  phase: Ready
`))
			f.RunHook()
		})

		It("Should set consoleLegacyCompat to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeFalse())
			Expect(f.ValuesGet("userAuthz.internal.consoleVersion").String()).To(Equal("v1.44.0"))
		})
	})

	Context("Console module is installed with newer version (v1.45.1)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties:
  version: v1.45.1
status:
  phase: Ready
`))
			f.RunHook()
		})

		It("Should set consoleLegacyCompat to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeFalse())
			Expect(f.ValuesGet("userAuthz.internal.consoleVersion").String()).To(Equal("v1.45.1"))
		})
	})

	Context("Console module exists but has no version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties: {}
status:
  phase: Ready
`))
			f.RunHook()
		})

		It("Should set consoleLegacyCompat to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeFalse())
		})
	})

	Context("Console module is updated from old to new version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties:
  version: v1.43.0
status:
  phase: Ready
`))
			f.RunHook()
		})

		It("Should initially set consoleLegacyCompat to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeTrue())
		})

		Context("Then console is updated to v1.44.0", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties:
  version: v1.44.0
status:
  phase: Ready
`))
				f.RunHook()
			})

			It("Should set consoleLegacyCompat to false", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeFalse())
			})
		})
	})

	Context("Console module is removed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: console
properties:
  version: v1.43.0
status:
  phase: Ready
`))
			f.RunHook()
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeTrue())

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should set consoleLegacyCompat to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.consoleLegacyCompat").Bool()).To(BeFalse())
			Expect(f.ValuesGet("userAuthz.internal.consoleVersion").Exists()).To(BeFalse())
		})
	})
})
