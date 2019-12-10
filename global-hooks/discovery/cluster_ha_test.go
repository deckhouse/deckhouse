/*

User-stories:
1. If Endpoint default/kubernetes has more than one addresses, someone took care of the cluster failover. Hook must figure out if we need to enable HA-mode for other modules.

*/

package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

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

var _ = Describe("Global hooks :: discovery/cluster_ha ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Endpoint default/kubernetes has single address in .subsets[]", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSingleAddress)...)
			f.RunHook()
		})

		It("Hook must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("BINDING_CONTEXT must contain Synchronization event with value '1' in filterResult", func() {
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
			Expect(f.BindingContexts[0].Binding).To(Equal("kube-api-ep"))
			Expect(f.BindingContexts[0].Type).To(Equal("Synchronization"))
			Expect(f.BindingContexts[0].Objects[0].FilterResult.String()).To(Equal("1"))
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false", func() {
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
		})

		Context("Someone added additional addresses to .subsets[]", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMultipleAddresses)...)
				f.RunHook()
			})

			It("Hook must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("BINDING_CONTEXT must contain Modified event with value '3' in filterResult", func() {
				Expect(f.BindingContexts).ShouldNot(BeEmpty())
				Expect(f.BindingContexts[0].Binding).To(Equal("kube-api-ep"))
				Expect(f.BindingContexts[0].WatchEvent).To(Equal("Modified"))
				Expect(f.BindingContexts[0].FilterResult.String()).To(Equal("3"))
			})

			It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be true", func() {
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})

		})

	})

	Context("Endpoint default/kubernetes has multiple addresses in .subsets[]", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMultipleAddresses)...)
			f.RunHook()
		})

		It("Hook must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("BINDING_CONTEXT must contain Synchronization event with value '3' in filterResult", func() {
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
			Expect(f.BindingContexts[0].Binding).To(Equal("kube-api-ep"))
			Expect(f.BindingContexts[0].Type).To(Equal("Synchronization"))
			Expect(f.BindingContexts[0].Objects[0].FilterResult.String()).To(Equal("3"))
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be true", func() {
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})

		Context("Someone set number of addresses in .subsets[] to one", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSingleAddress)...)
				f.RunHook()
			})

			It("Hook must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("BINDING_CONTEXT must contain Modified event with value '1' in filterResult", func() {
				Expect(f.BindingContexts).ShouldNot(BeEmpty())
				Expect(f.BindingContexts[0].Binding).To(Equal("kube-api-ep"))
				Expect(f.BindingContexts[0].WatchEvent).To(Equal("Modified"))
				Expect(f.BindingContexts[0].FilterResult.String()).To(Equal("1"))
			})

			It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false", func() {
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
			})
		})
	})
})
