package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: control-plane-manager :: helm template :: arguments secret", func() {

	const globalValues = `
  allocatableResources:
    internal:
      milliCpuControlPlane: 1024
      memoryControlPlane: 536870912
  clusterConfiguration:
    kubernetesVersion: 1.16.15
    clusterType: Cloud
  modules:
    placement: {}
  modulesImages:
    registry: registry.flant.com
    registryDockercfg: cfg
    tags:
      controlPlaneManager:
        controlPlaneManager: imagehash
        etcd: imagehash
        kubeApiserver116: imagehash
        kubeControllerManager116: imagehash
        kubeScheduler116: imagehash
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master:
        __ConstantChoices__: "3"
    nodeCountByType:
      cloud: 1
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.15.4
`
	const moduleValues = `
  internal:
    effectiveKubernetesVersion: "1.16"
    etcdServers:
      - https://192.168.199.186:2379
    pkiChecksum: checksum
    rolloutEpoch: 1857
`

	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSetFromYaml("controlPlaneManager", moduleValues)
	})

	Context("Two NGs with standby", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("controlPlaneManager.internal.arguments", `{"nodeStatusUpdateFrequency": "4s","nodeMonitorPeriod": "2s","nodeMonitorGracePeriod": "15s", "podEvictionTimeout": "15s", "defaultUnreachableTolerationSeconds": 15}`)
			f.HelmRender()
		})

		It("should render correctly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-control-plane-arguments")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field("data.arguments\\.json").String()).To(Equal("eyJkZWZhdWx0VW5yZWFjaGFibGVUb2xlcmF0aW9uU2Vjb25kcyI6MTUsIm5vZGVNb25pdG9yR3JhY2VQZXJpb2QiOiIxNXMiLCJub2RlTW9uaXRvclBlcmlvZCI6IjJzIiwibm9kZVN0YXR1c1VwZGF0ZUZyZXF1ZW5jeSI6IjRzIiwicG9kRXZpY3Rpb25UaW1lb3V0IjoiMTVzIn0="))
		})
	})
})
