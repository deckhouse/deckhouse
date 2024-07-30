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
	"fmt"
	"os"
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
	nsName = "d8-admission-policy-engine"

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
	f := SetupHelmConfig(`{"admissionPolicyEngine": {"denyVulnerableImages": {}, "podSecurityStandards": {}, "internal": {"podSecurityStandards": {"enforcementActions": ["deny"]}, "operationPolicies": [
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

	checkVWC := func(f *Config, webhooksCount int, rules ...string) {
		vw := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config")
		Expect(vw.Exists()).To(BeTrue())
		Expect(vw.Field("webhooks").Array()).To(HaveLen(webhooksCount))
		for i := 0; i < webhooksCount; i++ {
			Expect(vw.Field(fmt.Sprintf("webhooks.%d.rules", i)).String()).To(MatchJSON(rules[i]))
		}
	}

	BeforeSuite(func() {
		err := os.Symlink("/deckhouse/ee/modules/015-admission-policy-engine/templates/trivy-provider", "/deckhouse/modules/015-admission-policy-engine/templates/trivy-provider")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/modules/015-admission-policy-engine/templates/trivy-provider")
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, "admission-policy-engine")
			dp := f.KubernetesResource("Deployment", nsName, "gatekeeper-controller-manager")
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())

			tpSvc := f.KubernetesResource("Service", nsName, "trivy-provider")
			Expect(tpSvc.Exists()).To(BeFalse())

			vw := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config")
			Expect(vw.Exists()).To(BeFalse())
		})
	})

	Context("Cluster with deckhouse on master node with bootstrapped module", func() {
		BeforeEach(func() {
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.trackedConstraintResources", `[]`)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Renders empty ValidatingWebhookConfiguration", func() {
			checkVWC(f, 0)
		})
	})

	Context("Cluster with deckhouse on master node and trivy-provider", func() {
		trackedResourcesRules := `[{"apiGroups":[""],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["ingresses"]}]`
		trivyProviderRules := `[{"apiGroups":["apps"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["deployments","daemonsets","statefulsets"]},{"apiGroups":["apps.kruise.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["daemonsets"]},{"apiGroups":[""],"apiVersions":["*"],"operations":["CREATE"],"resources":["pods"]},{"apiGroups":[""],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["ingresses"]}]
		`

		BeforeEach(func() {
			f.ValuesSet("admissionPolicyEngine.denyVulnerableImages.enabled", true)
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
		})

		Context("disabled operator-trivy module", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("Everything must render properly", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("Doesn't create trivy-provider service", func() {
				tpSvc := f.KubernetesResource("Service", nsName, "trivy-provider")
				Expect(tpSvc.Exists()).To(BeFalse())
			})

			It("Creates ValidatingWebhookConfiguration after bootstrap", func() {
				checkVWC(f, 1, trackedResourcesRules)
			})
		})

		Context("enabled operator-trivy module", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler-crd", "prometheus", "operator-prometheus-crd", "operator-trivy"]`)
				f.ValuesSetFromYaml("admissionPolicyEngine.internal.denyVulnerableImages.webhook", `{"ca": "ca", "crt": "crt", "key": "key"}`)
				f.ValuesSetFromYaml("admissionPolicyEngine.internal.denyVulnerableImages.dockerConfigJson", `{"auths": {"registry.test.com": {"auth": "dXNlcjpwYXNzd29yZAo="}}}`)
				f.HelmRender()
			})

			It("Everything must render properly", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("Creates trivy-provider service", func() {
				tpSvc := f.KubernetesResource("Service", nsName, "trivy-provider")
				Expect(tpSvc.Exists()).To(BeTrue())
			})

			It("Registry secret stores data from values", func() {
				tpRegSecret := f.KubernetesResource("Secret", nsName, "trivy-provider-registry-secret")
				Expect(tpRegSecret.Exists()).To(BeTrue())
				Expect(tpRegSecret.Field(`data.config\.json`).String()).To(Equal("eyJhdXRocyI6eyJyZWdpc3RyeS50ZXN0LmNvbSI6eyJhdXRoIjoiZFhObGNqcHdZWE56ZDI5eVpBbz0ifX19"))
			})

			It("Creates ValidatingWebhookConfiguration after bootstrap with trivy provider config", func() {
				checkVWC(f, 2, trivyProviderRules, trackedResourcesRules)
			})
		})
	})
})
