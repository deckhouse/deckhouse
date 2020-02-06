package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: discover_apiserver_endpoints ::", func() {
	const (
		stateSingleAddress = `
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
  namespace: default
subsets:
- addresses:
  - ip: 10.0.3.192
`

		stateMultipleAddresses = `
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
  namespace: default
subsets:
- addresses:
  - ip: 10.0.3.192
  - ip: 10.0.3.193
  - ip: 10.0.3.194
`
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)

	Context("Endpoint default/kubernetes has single address in .subsets[]", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSingleAddress))
			f.RunHook()
		})

		It("`cloudInstanceManager.internal.clusterMasterAddresses` must be ['10.0.3.192']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.3.192"]`))
		})

		Context("Someone added additional addresses to .subsets[]", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMultipleAddresses))
				f.RunHook()
			})

			It("`cloudInstanceManager.internal.clusterMasterAddresses` must be ['10.0.3.192','10.0.3.193','10.0.3.194']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cloudInstanceManager.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.3.192","10.0.3.193","10.0.3.194"]`))
			})
		})
	})

	Context("Endpoint default/kubernetes has multiple addresses in .subsets[]", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMultipleAddresses))
			f.RunHook()
		})

		It("`cloudInstanceManager.internal.clusterMasterAddresses` must be ['10.0.3.192','10.0.3.193','10.0.3.194']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.3.192","10.0.3.193","10.0.3.194"]`))
		})

		Context("Someone set number of addresses in .subsets[] to one", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSingleAddress))
				f.RunHook()
			})

			It("`cloudInstanceManager.internal.clusterMasterAddresses` must be ['10.0.3.192']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cloudInstanceManager.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.3.192"]`))
			})
		})
	})
})
