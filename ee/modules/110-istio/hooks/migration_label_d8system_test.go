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

var _ = Describe("Istio hooks :: migration_label_d8system ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")

	Context("Application namespaces with labels and IstioOperator", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
  labels:
    aa: bb
    xx: yy
`))

			f.RunHook()
		})
		It("Should label d8-system properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-system").Field("metadata.labels").String()).To(MatchJSON(`{"aa":"bb","xx":"yy","istio.deckhouse.io/discovery":"disabled"}`))
		})
	})
})
