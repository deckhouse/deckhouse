/*
Copyright 2021 Flant JSC

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

package template_tests

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

var _ = Describe("Module :: control-plane-manager :: helm template :: arguments secret", func() {

	const globalValues = `
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: vSphere
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "Automatic"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  internal:
    modules:
      resourcesRequests:
        milliCpuControlPlane: 1024
        memoryControlPlane: 536870912
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master:
        __ConstantChoices__: "3"
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.15.4
`
	const moduleValues = `
  internal:
    effectiveKubernetesVersion: "1.29"
    etcdServers:
      - https://192.168.199.186:2379
    pkiChecksum: checksum
    rolloutEpoch: 1857
`

	f := SetupHelmConfig(`controlPlaneManager: {}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("controlPlaneManager", moduleValues)
	})

	Context("Prometheus rules", func() {
		assertSpecDotGroupsArray := func(rule object_store.KubeObject, length int) {
			Expect(rule.Exists()).To(BeTrue())

			groups := rule.Field("spec.groups")

			Expect(groups.IsArray()).To(BeTrue())
			Expect(groups.Array()).To(HaveLen(length))

		}

		Context("For etcd main", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus-crd"]`)
				f.HelmRender()
			})

			It("spec.groups should not be empty array", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				rule := f.KubernetesResource("PrometheusRule", "d8-system", "control-plane-manager-etcd-maintenance")

				assertSpecDotGroupsArray(rule, 1)
			})
		})
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

	Context("With secretEncryptionKey", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("controlPlaneManager.internal.secretEncryptionKey", `ABCDEFGHIJABCDEFGHIJABCDEFGHIJABCDEFGHIJABCD`)
			f.HelmRender()
		})

		It("should render correctly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
			Expect(s.Exists()).To(BeTrue())
			data, err := base64.StdEncoding.DecodeString(s.Field("data.extra-file-secret-encryption-config\\.yaml").String())
			Expect(err).To(BeNil())
			Expect(data).To(MatchYAML(`
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
    - secrets
    providers:
    - aescbc:
        keys:
        - name: secretbox
          secret: ABCDEFGHIJABCDEFGHIJABCDEFGHIJABCDEFGHIJABCD
    - identity: {}
`))
		})
	})

})
