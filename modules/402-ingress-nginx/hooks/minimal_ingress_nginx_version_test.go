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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: minimal_ingress_version ", func() {
	initValuesString := `{"ingressNginx":{"defaultControllerVersion": "0.33", "internal": {}}}`
	globalValuesString := `{}`
	f := HookExecutionConfigInit(initValuesString, globalValuesString)
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("IngressNginxController CR does not exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should have no minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(minVersionValuesKey).Exists()).To(BeFalse())
		})
	})

	Context("Only one IngressNginxController CR exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  controllerVersion: "1.1"
`))
			f.RunHook()
		})

		It("Should have minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(minVersionValuesKey).String()).To(BeEquivalentTo("1.1.0"))
		})
	})

	Context("Few IngressNginxController CR exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: first
spec:
  controllerVersion: "1.1"
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: second
spec:
  controllerVersion: "0.33"
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: third
spec:
  controllerVersion: "0.46"
`))
			f.RunHook()
		})

		It("Should have minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(minVersionValuesKey).String()).To(BeEquivalentTo("0.33.0"))
		})
	})
})
