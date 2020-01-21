/*

User-stories:
1. There is CM kube-system/extension-apiserver-authentication with CA for verification requests to our custom modules from clients inside cluster, hook must store it to `global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_dns_address ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateA = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-apiserver-authentication
  namespace: kube-system
data:
  requestheader-client-ca-file: |
    qraga
`
		stateB = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-apiserver-authentication
  namespace: kube-system
data:
  requestheader-client-ca-file: |
    pickle
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster started with some extension-apiserver-authentication", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("clientCA must be 'qraga'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA").String()).To(Equal("qraga"))
		})

		Context("extension-apiserver-authentication changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.RunHook()
			})

			It("clientCA must be 'pickle'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA").String()).To(Equal("pickle"))
			})

		})

	})
})
