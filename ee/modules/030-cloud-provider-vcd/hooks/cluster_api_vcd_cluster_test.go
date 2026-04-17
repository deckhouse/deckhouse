/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: cluster_api_vcd_cluster ::", func() {
	const (
		vcdCluster = `
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: VCDCluster
metadata:
  name: test-cluster
  namespace: d8-cloud-instance-manager
status:
  ready: true
`
		cluster = `
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: test-cluster
  namespace: d8-cloud-instance-manager
spec:
  infrastructureRef:
    apiGroup: infrastructure.cluster.x-k8s.io
    kind: VCDCluster
    name: test-cluster
`
		clusterWithStatus = `
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: test-cluster
  namespace: d8-cloud-instance-manager
spec:
  infrastructureRef:
    apiGroup: infrastructure.cluster.x-k8s.io
    kind: VCDCluster
    name: test-cluster
status:
  initialization:
    infrastructureProvisioned: true
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.29.0", "kubernetesVersions":["1.29.0"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"cloudProviderVcd":{"internal": {}}}`, `{}`)
	f.RegisterCRD("infrastructure.cluster.x-k8s.io", "v1beta2", "VCDCluster", false)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta2", "Cluster", false)

	Context("VCDCluster ready and Cluster without status", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(vcdCluster+cluster, 2))
			f.RunHook()
		})

		It("Hook must not fail and set infrastructureProvisioned", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "test-cluster").ToYaml()).To(MatchYAML(clusterWithStatus))
		})
	})

	Context("VCDCluster not ready", func() {
		BeforeEach(func() {
			vcdClusterNotReady := `
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: VCDCluster
metadata:
  name: test-cluster
  namespace: d8-cloud-instance-manager
status:
  ready: false
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(vcdClusterNotReady+cluster, 2))
			f.RunHook()
		})

		It("Hook must not fail and not patch Cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "test-cluster").ToYaml()).To(MatchYAML(cluster))
		})
	})
})
