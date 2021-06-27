/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Basic Auth :: migrate_service ::", func() {
	const (
		initValues       = `{}`
		initConfigValues = `{}`

		notMigratedService = `
apiVersion: v1
kind: Service
metadata:
  name: basic-auth
  namespace: kube-basic-auth
spec:
  clusterIP: None
`

		migratedService = `
apiVersion: v1
kind: Service
metadata:
  name: basic-auth
  namespace: kube-basic-auth
spec:
  clusterIP: 192.168.150.132
`
	)

	f := HookExecutionConfigInit(initValues, initConfigValues)

	Context("Fresh cluster without a Service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with a Service for migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(notMigratedService))
			f.RunHook()
		})

		It("Hook must not fail, Service should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Service", "kube-basic-auth", "basic-auth").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with a migrated Service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedService))
			f.RunHook()
		})

		It("Hook must not fail, Service should be present", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Service", "kube-basic-auth", "basic-auth").Exists()).To(BeTrue())
		})
	})

})
