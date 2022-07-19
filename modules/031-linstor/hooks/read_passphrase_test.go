/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: linstor :: hooks :: read_passphrase ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Master passphase :: Secret missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(``),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Master passphrase should not be specified in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.masterPassphrase").Exists()).To(BeFalse())
		})
	})

	Context("Master passphrase :: Secret does not contain data", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-passphrase
  namespace: d8-system
data:
  foo: YmFy
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Master passphrase stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.masterPassphrase").Exists()).To(BeFalse())
		})
	})

	Context("Master passphrase :: Passphrase is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-passphrase
  namespace: d8-system
data:
  MASTER_PASSPHRASE: ""
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Master passphrase stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.masterPassphrase").Exists()).To(BeFalse())
		})
	})

	Context("Master passphrase :: Secret created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-passphrase
  namespace: d8-system
data:
  MASTER_PASSPHRASE: aGFja21l
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Master passphrase stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.masterPassphrase").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.masterPassphrase").String()).To(Equal("hackme"))
		})
	})

	fb := HookExecutionConfigInit(`{"linstor":{"internal":{"masterPassphrase": "abcdef"}}}`, initConfigValuesString)
	Context("Master passphrase :: Passphrase removal", func() {
		BeforeEach(func() {
			fb.BindingContexts.Set(
				fb.KubeStateSet(``), fb.GenerateBeforeHelmContext())
			fb.RunHook()
		})

		It("Master passphrase removed from values", func() {
			Expect(fb).To(ExecuteSuccessfully())
			Expect(fb.ValuesGet("linstor.internal.masterPassphrase").Exists()).To(BeFalse())
		})
	})
})
