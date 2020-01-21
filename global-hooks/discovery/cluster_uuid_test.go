/*

User-stories:
1. There is CM kube-system/d8-cluster-uuid with cluster uuid. Hook must store it to `global.discovery.clusterUUID`.
2. There isn't CM kube-system/d8-cluster-uuid. Hook must generate new UUID, store it to `global.discovery.clusterUUID` and create CM with it.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: cluster_uuid ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateCM = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-uuid
  namespace: kube-system
data:
  cluster-uuid: 2528b7ff-a5eb-48d1-b0b0-4c87628284de
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("objects must be 'empty'; `global.discovery.clusterUUID` must be generated, ", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Get("0.objects.0").Array()).To(BeEmpty())
			newUUID := f.ValuesGet("global.discovery.clusterUUID").String()
			Expect(len(newUUID)).To(Equal(36))
			Expect(f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-uuid").Field("data.cluster-uuid").String()).To(Equal(newUUID))
		})

		Context("CM d8-cluster-uuid created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateCM))
				f.RunHook()
			})

			It("filterResult and global.discovery.clusterUUID must be '2528b7ff-a5eb-48d1-b0b0-4c87628284de'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Get("0.filterResult").String()).To(Equal("2528b7ff-a5eb-48d1-b0b0-4c87628284de"))
				Expect(f.ValuesGet("global.discovery.clusterUUID").String()).To(Equal("2528b7ff-a5eb-48d1-b0b0-4c87628284de"))
			})
		})
	})

	Context("CM d8-cluster-uuid exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCM))
			f.RunHook()
		})

		It("filterResult and global.discovery.clusterUUID must be '2528b7ff-a5eb-48d1-b0b0-4c87628284de'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").String()).To(Equal("2528b7ff-a5eb-48d1-b0b0-4c87628284de"))
			Expect(f.ValuesGet("global.discovery.clusterUUID").String()).To(Equal("2528b7ff-a5eb-48d1-b0b0-4c87628284de"))
		})

		Context("CM d8-cluster-uuid deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("Hook must fail", func() {
				Expect(f).To(Not(ExecuteSuccessfully()))
			})
		})
	})
})
