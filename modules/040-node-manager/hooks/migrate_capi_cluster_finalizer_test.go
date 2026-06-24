/*
Copyright 2026 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: migrate_capi_cluster_finalizer ::", func() {
	const (
		clusterWithCustomFinalizer = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test
  namespace: d8-cloud-instance-manager
  finalizers:
    - cluster.cluster.x-k8s.io
    - deckhouse.io/capi-controller-manager
spec: {}
`
		clusterWithoutCustomFinalizer = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test
  namespace: d8-cloud-instance-manager
  finalizers:
    - cluster.cluster.x-k8s.io
spec: {}
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Cluster", false)

	Context("Cluster with custom node-manager finalizer", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(clusterWithCustomFinalizer, 1))
			f.RunHook()
		})

		It("removes only the custom finalizer", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "test")
			Expect(cluster.Exists()).To(BeTrue())
			Expect(cluster.ToYaml()).To(MatchYAML(clusterWithoutCustomFinalizer))
		})
	})

	Context("Cluster without custom node-manager finalizer", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(clusterWithoutCustomFinalizer, 1))
			f.RunHook()
		})

		It("keeps the cluster unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "test")
			Expect(cluster.Exists()).To(BeTrue())
			Expect(cluster.ToYaml()).To(MatchYAML(clusterWithoutCustomFinalizer))
		})
	})
})
