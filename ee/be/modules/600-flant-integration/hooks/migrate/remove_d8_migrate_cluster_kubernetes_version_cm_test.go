/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package migrate

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{}`
	initConfigValuesString = `{}`
)

var _ = Describe("Module hooks :: d8-flant-integration :: remove migration configmap", func() {

	const configMapTemplate = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-migrate-cluster-kubernetes-version
  namespace: d8-flant-integration
`

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Configmap about migration absent", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "d8-flant-integration", "d8-migrate-cluster-kubernetes-version")
			Expect(cm.Exists()).To(BeFalse())
		})
	})

	Context("Configmap about migration present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(configMapTemplate))
			f.RunHook()
		})

		It("Hook should run, configmap should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "d8-flant-integration", "d8-migrate-cluster-kubernetes-version")
			Expect(cm.Exists()).To(BeFalse())
		})
	})
})
