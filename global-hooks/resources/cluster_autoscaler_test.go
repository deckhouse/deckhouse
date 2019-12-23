/*

User-stories:
1. If there is a Deployment kube-system/cluster-autoscaler in cluster, it must not have section `resources.limits` because extended-monitoring will alert at throttling.

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
	initValuesString       = `{}`
	initConfigValuesString = `{}`
)

const (
	stateEmpty = ``

	stateLimitsAreSet = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  template:
    spec:
      containers:
      - resources:
          requests:
            cpu: 100m
            memory: 300Mi
          limits:
            cpu: 333m
            memory: 333Mi`

	stateLimitsAreUnset = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  template:
    spec:
      containers:
      - resources:
          requests:
            cpu: 100m
            memory: 300Mi`
)

var _ = Describe("Global hooks :: resources/cluster_autoscaler ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("There is no Deployment kube-system/cluster-autoscaler in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateEmpty)...)
			f.RunHook()
		})

		It("BINDING_CONTEXT must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
			Expect(f.BindingContexts[0].Objects).To(BeEmpty())
		})

		Context("Someone created Deployment kube-system/cluster-autoscaler with `spec.template.spec.containers.0.resources.limits`", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreSet)...)
				f.RunHook()
			})

			It("BINDING_CONTEXT must contain Added event; section `limits` must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts).ShouldNot(BeEmpty())
				Expect(f.BindingContexts[0].Binding).To(Equal("cluster-autoscaler"))
				Expect(f.BindingContexts[0].WatchEvent).To(Equal("Added"))
				Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
			})
		})
	})

	Context("There is Deployment kube-system/cluster-autoscaler in cluster with section `spec.template.spec.containers.0.resources.limits`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreSet)...)
			f.RunHook()
		})

		It("BINDING_CONTEXT must contain Synchronization event with cluster-autoscaler Deployment; section `limits` must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
			Expect(f.BindingContexts[0].Binding).To(Equal("cluster-autoscaler"))
			Expect(f.BindingContexts[0].Type).To(Equal("Synchronization"))
			Expect(f.BindingContexts[0].Objects[0].Object.Field("metadata.name").String()).To(Equal("cluster-autoscaler"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
		})
	})

	Context("There is Deployment kube-system/cluster-autoscaler in cluster without section `spec.template.spec.containers.0.resources.limits`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreUnset)...)
			f.RunHook()
		})

		It("BINDING_CONTEXT must contain Synchronization event with cluster-autoscaler Deployment; section `limits` must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
			Expect(f.BindingContexts[0].Binding).To(Equal("cluster-autoscaler"))
			Expect(f.BindingContexts[0].Type).To(Equal("Synchronization"))
			Expect(f.BindingContexts[0].Objects[0].Object.Field("metadata.name").String()).To(Equal("cluster-autoscaler"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
		})

		Context("Someone modified Deployment kube-system/cluster-autoscaler by adding section `spec.template.spec.containers.0.resources.limits`", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreSet)...)
				f.RunHook()
			})

			It("BINDING_CONTEXT must contain Modified event; section `limits` must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts).ShouldNot(BeEmpty())
				Expect(f.BindingContexts[0].Binding).To(Equal("cluster-autoscaler"))
				Expect(f.BindingContexts[0].WatchEvent).To(Equal("Modified"))
				Expect(f.BindingContexts[0].Object.Field("metadata.name").String()).To(Equal("cluster-autoscaler"))
				Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler").Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
			})
		})
	})
})
