package hooks

/*

User-stories:
1. There is Service kube-system/kube-dns with clusterIP, hook must store it to `global.discovery.clusterDNSAddress`.

*/

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
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  clusterIP: 192.168.0.10
`
		stateB = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  clusterIP: 192.168.0.42
`
		stateC = `
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  type: ExternalName
  externalName: d8-kube-dns.kube-system.svc.cluster.local
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

			Context("Adding CNAME service without clusterIP", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateB + stateC))
					f.RunHook()
				})

				It("global.discovery.clusterDNSAddress must be '192.168.0.42'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.42"))
				})
			})
		})
	})

	Context("Fresh cluster without dns service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("should fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})
})
