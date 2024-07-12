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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: minimal_ingress_version ", func() {
	initValuesString := `{"ingressNginx":{"defaultControllerVersion": "1.6", "internal": {}}}`
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
			_, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeFalse())
			v, _ := requirements.GetValue(incompatibleVersionsKey)
			Expect(v).To(BeFalse())
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
  controllerVersion: "1.6"
  ingressClass: "nginx"
`))
			f.RunHook()
		})

		It("Should have minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("1.6.0"))
			v, _ := requirements.GetValue(incompatibleVersionsKey)
			Expect(v).To(BeFalse())
		})
	})

	Context("IngressNginxController with default version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
`))
			f.RunHook()
		})

		It("Should have minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("1.6.0"))
			v, _ := requirements.GetValue(incompatibleVersionsKey)
			Expect(v).To(BeFalse())
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
  controllerVersion: "1.6"
  ingressClass: "test"
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: second
spec:
  controllerVersion: "0.33"
  ingressClass: "test2"
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: third
spec:
  controllerVersion: "0.46"
  ingressClass: "nginx"
`))
			f.RunHook()
		})

		It("Should have minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("0.33.0"))
			v, _ := requirements.GetValue(incompatibleVersionsKey)
			Expect(v).To(BeFalse())
		})
	})

	Context("Has incompatible ingress controllers", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: first
spec:
  controllerVersion: "1.6"
  ingressClass: "test"
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: second
spec:
  controllerVersion: "0.33"
  ingressClass: "test"
`))
			f.RunHook()
		})

		It("Should have minimal version", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("0.33.0"))
			v, _ := requirements.GetValue(incompatibleVersionsKey)
			Expect(v).To(BeTrue())
		})
	})
})
