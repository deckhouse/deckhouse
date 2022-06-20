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

var _ = Describe("Istio hooks :: discovery_operator_revisions_to_install ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			values := `
internal:
  supportedVersions: ["1.1","1.2.3-beta.45"]
  revisionsToInstall: ["v1x1"]
`
			f.ValuesSetFromYaml("istio", []byte(values))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.operatorRevisionsToInstall").String()).To(MatchJSON(`["v1x1"]`))
		})
	})

	Context("There are supported IstioOperators in cluster", func() {
		BeforeEach(func() {
			values := `
internal:
  supportedVersions: ["1.1.0", "1.8.0-alpha.2", "1.2", "1.3", "1.4"]
  revisionsToInstall: ["v1x3", "v1x4"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x8x0alpha2
  namespace: d8-istio
spec:
  revision: v1x8x0alpha2
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
			Expect(f.ValuesGet("istio.internal.operatorRevisionsToInstall").AsStringSlice()).To(Equal([]string{"v1x2", "v1x3", "v1x4", "v1x8x0alpha2"}))
		})
	})

	Context("There are unsupported IstioOperators in cluster", func() {
		BeforeEach(func() {
			values := `
internal:
  supportedVersions: ["1.1.0", "1.8.0-alpha.2", "1.2", "1.3", "1.4"]
  revisionsToInstall: ["v1x3", "v1x4"]
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x9x0-bad
  namespace: d8-istio
spec:
  revision: v1x9x0
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

			Expect(f.GoHookError).To(MatchError("unsupported revisions: [v1x0,v1x9x0]"))
		})
	})
})
