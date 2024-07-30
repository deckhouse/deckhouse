/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

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
	initValuesString       = `{"nodeLocalDns": {"internal": {}}}`
	initConfigValuesString = `{}`
)

var _ = Describe("Global hooks :: discovery :: cluster_dns_address ::", func() {
	const (
		redirectState = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns-redirect
  namespace: kube-system
  labels:
    app: coredns-redirect
spec:
  clusterIP: 192.168.0.20
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster started with clusterIP = '192.168.0.20'", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(redirectState))
			f.RunHook()
		})

		It("nodeLocalDns.internal.clusterDNSRedirectAddress must be '192.168.0.20'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeLocalDns.internal.clusterDNSRedirectAddress").String()).To(Equal("192.168.0.20"))
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
