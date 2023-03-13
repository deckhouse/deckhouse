/*
Copyright 2022 Flant JSC

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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus", "operator-prometheus-crd"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`
)

var _ = Describe("Module :: admissionPolicyEngine :: helm template ::", func() {
	f := SetupHelmConfig(`{"admissionPolicyEngine": {podSecurityStandards: {}, internal: {"operationPolicies": [
    {
      "metadata": {
        "name": "foo"
      },
      "spec": {
        "enforcementAction": "Deny",
        "match": {
          "labelSelector": {
            "matchLabels": {
              "operation-policy.deckhouse.io/enabled": "true"
            }
          },
          "namespaceSelector": {
            "excludeNames": [
              "some-ns"
            ],
            "labelSelector": {
              "matchLabels": {
                "operation-policy.deckhouse.io/enabled": "true"
              }
            },
            "matchNames": [
              "default"
            ]
          }
        },
        "policies": {
          "allowedRepos": [
            "foo"
          ]
        }
      }
    }
  ],
	trackedConstraintResources: [{"apiGroups":[""],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}],
	trackedMutateResources: [{"apiGroups":[""],"resources":["pods"]}],
	webhook: {ca: YjY0ZW5jX3N0cmluZwo=, crt: YjY0ZW5jX3N0cmluZwo=, key: YjY0ZW5jX3N0cmluZwo=}}}}`)

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		nsName := "d8-admission-policy-engine"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, "admission-policy-engine")
			dp := f.KubernetesResource("Deployment", nsName, "gatekeeper-controller-manager")
			vw := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config")
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(vw.Exists()).To(BeFalse())
		})
	})
})
