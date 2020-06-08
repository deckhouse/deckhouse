/*

User-stories:
1. Hook must discover number of addresses in Endpoint default/kubernetes and save to global.discovery.clusterMasterCount,
2. If number of addresses in Endpoint default/kubernetes is more than one — hook must set global.discovery.clusterControlPlaneIsHighlyAvailable to true, else — to false.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_ha ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateSingleAddress = `
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
  namespace: default
subsets:
- addresses:
  - ip: 10.0.3.192`

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
  - ip: 10.0.3.194`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Endpoint default/kubernetes has single address in .subsets[]", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSingleAddress))
			f.RunHook()
		})

		It("filterResult.isHA must be false;  filterResult.clusterMasterCount must be 1; `global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` must be 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.binding").String()).To(Equal("kube_api_ep"))
			Expect(f.BindingContexts.Get("0.type").String()).To(Equal("Synchronization"))
			Expect(f.BindingContexts.Get("0.objects.0.filterResult.clusterMasterCount").String()).To(Equal("1"))
			Expect(f.BindingContexts.Get("0.objects.0.filterResult.isHA").Bool()).To(BeFalse())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})

		Context("Someone added additional addresses to .subsets[]", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMultipleAddresses))
				f.RunHook()
			})

			It("filterResult.isHA must be true;  filterResult.clusterMasterCount must be 3; `global.discovery.clusterControlPlaneIsHighlyAvailable` must be true; `global.discovery.clusterMasterCount` must be 3", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
				Expect(f.BindingContexts.Get("0.binding").String()).To(Equal("kube_api_ep"))
				Expect(f.BindingContexts.Get("0.watchEvent").String()).To(Equal("Modified"))
				Expect(f.BindingContexts.Get("0.filterResult.clusterMasterCount").String()).To(Equal("3"))
				Expect(f.BindingContexts.Get("0.filterResult.isHA").Bool()).To(BeTrue())

				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("3"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})
	})

	Context("Endpoint default/kubernetes has multiple addresses in .subsets[]", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMultipleAddresses))
			f.RunHook()
		})

		It("filterResult.isHA must be true;  filterResult.clusterMasterCount must be 3; `global.discovery.clusterControlPlaneIsHighlyAvailable` must be true; `global.discovery.clusterMasterCount` must be 3", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.binding").String()).To(Equal("kube_api_ep"))
			Expect(f.BindingContexts.Get("0.type").String()).To(Equal("Synchronization"))
			Expect(f.BindingContexts.Get("0.objects.0.filterResult.clusterMasterCount").String()).To(Equal("3"))
			Expect(f.BindingContexts.Get("0.objects.0.filterResult.isHA").Bool()).To(BeTrue())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("3"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})

		Context("Someone set number of addresses in .subsets[] to one", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSingleAddress))
				f.RunHook()
			})

			It("filterResult.isHA must be false;  filterResult.clusterMasterCount must be 1; `global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` must be 1", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
				Expect(f.BindingContexts.Get("0.binding").String()).To(Equal("kube_api_ep"))
				Expect(f.BindingContexts.Get("0.watchEvent").String()).To(Equal("Modified"))
				Expect(f.BindingContexts.Get("0.filterResult.clusterMasterCount").String()).To(Equal("1"))
				Expect(f.BindingContexts.Get("0.filterResult.isHA").Bool()).To(BeFalse())

				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
			})
		})
	})
})
