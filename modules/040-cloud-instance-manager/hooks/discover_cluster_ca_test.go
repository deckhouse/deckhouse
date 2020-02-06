package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: discover_cluster_ca ::", func() {
	const (
		stateA = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-apiserver-authentication
  namespace: kube-system
data:
  client-ca-file: |
    qraga
`
		stateB = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-apiserver-authentication
  namespace: kube-system
data:
  client-ca-file: |
    pickle
`
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)

	Context("Cluster started with some extension-apiserver-authentication", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("clusterCA must be 'qraga'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.clusterCA").String()).To(Equal("qraga"))
		})

		Context("extension-apiserver-authentication changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.RunHook()
			})

			It("clientCA must be 'pickle'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cloudInstanceManager.internal.clusterCA").String()).To(Equal("pickle"))
			})
		})
	})
})
