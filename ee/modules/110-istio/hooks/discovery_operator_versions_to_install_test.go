/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
     "1.1":
        revision: "v1x1"
     "1.2":
        revision: "v1x2"
  versionsToInstall: ["1.1"]
`
			f.ValuesSetFromYaml("istio", []byte(values))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))

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
    "1.8":
        revision: "v1x8"
    "1.2":
        revision: "v1x2"
    "1.3":
        revision: "v1x3"
    "1.4":
        revision: "v1x4"
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

	Context("There are unsupported IstioOperators in cluster", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap:
    "1.1":
        revision: "v1x1"
    "1.8":
        revision: "v1x8"
    "1.2":
        revision: "v1x2"
    "1.3":
        revision: "v1x3"
    "1.4":
        revision: "v1x4"
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
})
