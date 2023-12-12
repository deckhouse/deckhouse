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
	initValuesString       = `{"cloudProviderVcd": {"internal": {}}}`
	initConfigValuesString = `{}`
)

var _ = Describe("cloudProviderVcd :: api server discovery", func() {
	const (
		svc = `
---
apiVersion: v1
kind: Service
metadata:
  name: kubernetes
  namespace: default
spec:
  clusterIP: 192.168.0.1
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Service ip is '192.168.0.1'", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(svc))
			f.RunHook()
		})

		It("cloudProviderVcd.internal.apiServerIp must be '192.168.0.20'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderVcd.internal.apiServerIp").String()).To(Equal("192.168.0.1"))
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
