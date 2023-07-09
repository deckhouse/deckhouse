/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: handle_capi_clusters ::", func() {
	const (
		clusterWithoutInfrastructureRef = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev1
  namespace: d8-cloud-instance-manager
status:
  infrastructureReady: false
`
		clusterOpenstack1 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev1
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha6
    kind: OpenStackCluster
    name: dev1
    namespace: d8-cloud-instance-manager
status:
  infrastructureReady: false
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha6
kind: OpenStackCluster
metadata:
  name: dev1
  namespace: d8-cloud-instance-manager
`
		clusterOpenstack2 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev2
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha6
    kind: OpenStackCluster
    name: dev2
    namespace: d8-cloud-instance-manager
status:
  infrastructureReady: true
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha6
kind: OpenStackCluster
metadata:
  name: dev2
  namespace: d8-cloud-instance-manager
`
		clusterOpenstack3 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev3
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha6
    kind: OpenStackCluster
    name: dev3
    namespace: d8-cloud-instance-manager
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha6
kind: OpenStackCluster
metadata:
  name: dev3
  namespace: d8-cloud-instance-manager
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Cluster", true)
	f.RegisterCRD("infrastructure.cluster.x-k8s.io", "v1alpha6", "OpenStackCluster", true)
	f.RegisterCRD("ovirtproviderconfig.machine.openshift.io", "v1beta1", "OvirtClusterProviderSpec", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("More than one cluster resource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterOpenstack1 + clusterOpenstack2 + clusterOpenstack3))
			f.RunHook()
		})

		It("clusters status infrastructure state should be true and ownerRef on infrastructure cluster should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster1 := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev1")
			cluster2 := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev2")
			cluster3 := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev3")
			infraCluster1 := f.KubernetesResource("OpenStackCluster", "d8-cloud-instance-manager", "dev1")
			infraCluster2 := f.KubernetesResource("OpenStackCluster", "d8-cloud-instance-manager", "dev2")
			infraCluster3 := f.KubernetesResource("OpenStackCluster", "d8-cloud-instance-manager", "dev3")

			Expect(cluster1.Exists()).To(BeTrue())
			Expect(cluster1.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(infraCluster1.Field("metadata.ownerReferences")).To(MatchYAML(`
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: dev1
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
`))

			Expect(cluster2.Exists()).To(BeTrue())
			Expect(cluster2.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(infraCluster2.Field("metadata.ownerReferences")).To(MatchYAML(`
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: dev2
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
`))

			Expect(cluster3.Exists()).To(BeTrue())
			Expect(cluster3.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(infraCluster3.Field("metadata.ownerReferences")).To(MatchYAML(`
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: dev3
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
`))

		})
	})

	Context("cluster resource without infrastructureRef field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterWithoutInfrastructureRef))
			f.RunHook()
		})

		It("Must be executed successfully, cluster resource should not be chahged", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev1")

			Expect(cluster.Exists()).To(BeTrue())
			Expect(cluster.ToYaml()).To(MatchYAML(clusterWithoutInfrastructureRef))
		})
	})

	Context("Openstack: update statuses (infrastructureReady = false)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterOpenstack1))
			f.RunHook()
		})

		It("Clusters status infrastructure state should be true and ownerRef on infrastructure cluster should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev1")
			infraCluster := f.KubernetesResource("OpenStackCluster", "d8-cloud-instance-manager", "dev1")

			Expect(cluster.Exists()).To(BeTrue())
			Expect(cluster.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(infraCluster.Field("metadata.ownerReferences")).To(MatchYAML(`
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: dev1
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
`))

		})
	})

	Context("Openstack: update statuses (infrastructureReady = true)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterOpenstack2))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("clusters status infrastructure state should be true and ownerRef on infrastructure cluster should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev2")
			infraCluster := f.KubernetesResource("OpenStackCluster", "d8-cloud-instance-manager", "dev2")

			Expect(cluster.Exists()).To(BeTrue())
			Expect(cluster.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(infraCluster.Field("metadata.ownerReferences")).To(MatchYAML(`
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: dev2
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
`))
		})
	})

	Context("Openstack: update statuses (infrastructureReady is absent)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterOpenstack3))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("clusters status infrastructure state should be true and ownerRef on infrastructure cluster should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev3")
			infraCluster := f.KubernetesResource("OpenStackCluster", "d8-cloud-instance-manager", "dev3")

			Expect(cluster.Exists()).To(BeTrue())
			Expect(cluster.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(infraCluster.Field("metadata.ownerReferences")).To(MatchYAML(`
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: dev3
  namespace: d8-cloud-instance-manager
  uid: 123-456-789
`))

		})
	})

})
