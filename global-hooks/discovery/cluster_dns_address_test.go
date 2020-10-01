/*

User-stories:
1. There is Service kube-system/kube-dns with clusterIP, hook must store it to `global.discovery.clusterDNSAddress`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"global": {"discovery": {}}}`
	initConfigValuesString = `{}`
)

var _ = Describe("Global hooks :: discovery/cluster_dns_address ::", func() {
	const (
		stateA = `
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  clusterIP: 192.168.0.10
`
		stateB = `
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  clusterIP: 192.168.0.42
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster started with clusterIP = '192.168.0.10'", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("global.discovery.clusterDNSAddress must be '192.168.0.10'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.10"))
		})

		Context("clusterIP changed to 192.168.0.42", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.RunHook()
			})

			It("global.discovery.clusterDNSAddress must be '192.168.0.42'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.42"))
			})
		})
	})
})
