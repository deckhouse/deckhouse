/*
Copyright 2023 Flant JSC

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

var _ = Describe("Istio hooks :: discovery_operator_versions_to_install ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)
	f.RegisterCRD("sailoperator.io", "v1", "Istio", true)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
     "1.1":
        revision: "v1x1"
        supportsOperator: true
     "1.2":
        revision: "v1x2"
        supportsOperator: true
  versionsToInstall: ["1.1"]
`
			f.ValuesSetFromYaml("istio", []byte(values))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LoggerOutput.Contents()).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.operatorVersionsToInstall").String()).To(MatchJSON(`["1.1"]`))
		})
	})

	Context("There are supported IstioOperators in cluster", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.1":
        revision: "v1x1"
        supportsOperator: true
    "1.8":
        revision: "v1x8"
        supportsOperator: true
    "1.2":
        revision: "v1x2"
        supportsOperator: true
    "1.3":
        revision: "v1x3"
        supportsOperator: true
    "1.4":
        revision: "v1x4"
        supportsOperator: true
  versionsToInstall: ["1.3", "1.4"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x8
  namespace: d8-istio
spec:
  revision: v1x8
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x2
  namespace: d8-istio
spec:
  revision: v1x2
`))

			f.RunHook()
		})
		It("Should count all namespaces and revisions properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.operatorVersionsToInstall").AsStringSlice()).To(Equal([]string{"1.2", "1.3", "1.4", "1.8"}))
		})
	})

	Context("There are supported Istio resources in cluster", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.1":
        revision: "v1x1"
        supportsOperator: true
    "1.8":
        revision: "v1x8"
        supportsOperator: true
    "1.2":
        revision: "v1x2"
        supportsOperator: true
    "1.3":
        revision: "v1x3"
        supportsOperator: true
    "1.4":
        revision: "v1x4"
        supportsOperator: true
  versionsToInstall: ["1.3", "1.4"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: v1x8
  namespace: d8-istio
spec:
  revision: v1x8
---
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: v1x2
  namespace: d8-istio
spec:
  revision: v1x2
`))

			f.RunHook()
		})
		It("Should include versions from Istio resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.operatorVersionsToInstall").AsStringSlice()).To(Equal([]string{"1.2", "1.3", "1.4", "1.8"}))
		})
	})

	Context("There are unsupported IstioOperators in cluster", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.1":
        revision: "v1x1"
        supportsOperator: true
    "1.8":
        revision: "v1x8"
        supportsOperator: true
    "1.2":
        revision: "v1x2"
        supportsOperator: true
    "1.3":
        revision: "v1x3"
        supportsOperator: true
    "1.4":
        revision: "v1x4"
        supportsOperator: true
  versionsToInstall: ["1.3", "1.4"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x9-bad
  namespace: d8-istio
spec:
  revision: v1x9
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x0-bad
  namespace: d8-istio
spec:
  revision: v1x0
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x2
  namespace: d8-istio
spec:
  revision: v1x2
`))

			f.RunHook()
		})

		It("Should return errors", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("unsupported revisions: [v1x0,v1x9]"))
		})
	})

	Context("Operator-free versions are excluded from operatorVersionsToInstall", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.25":
        revision: "v1x25"
        supportsOperator: true
    "1.27":
        revision: "v1x27"
        supportsOperator: false
  versionsToInstall: ["1.25", "1.27"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should include only operator-supported versions", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.operatorVersionsToInstall").AsStringSlice()).To(Equal([]string{"1.25"}))
		})
	})

	Context("Operator-free IstioOperator in cluster is ignored", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.27":
        revision: "v1x27"
        supportsOperator: false
  versionsToInstall: []
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x27
  namespace: d8-istio
spec:
  revision: v1x27
`))
			f.RunHook()
		})

		It("Should not add operator-free version from CRD", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.operatorVersionsToInstall").AsStringSlice()).To(BeEmpty())
		})
	})

	Context("Unknown versions in versionsToInstall are skipped", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.25":
        revision: "v1x25"
        supportsOperator: true
  versionsToInstall: ["1.25", "1.30"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should keep only known operator-supported versions", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.operatorVersionsToInstall").AsStringSlice()).To(Equal([]string{"1.25"}))
		})
	})
})
