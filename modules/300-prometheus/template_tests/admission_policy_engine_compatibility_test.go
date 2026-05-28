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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: prometheus :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", `
enabledModules: ["vertical-pod-autoscaler", "prometheus", "admission-policy-engine", "admission-policy-engine-crd"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("prometheus", `
auth: {}
vpa: {}
grafana: {}
https:
  mode: CustomCertificate
internal:
  customCertificateData:
    tls.crt: |
      -----BEGIN CERTIFICATE-----
      TEST
      -----END CERTIFICATE-----
    tls.key: |
      -----BEGIN PRIVATE KEY-----
      TEST
      -----END PRIVATE KEY-----
  alertmanagers:
    byAddress: []
    byService: []
    internal: []
  auth: {}
  deployDexAuthenticator: true
  grafana:
    enabled: true
    additionalDatasources: []
    alertsChannelsConfig:
      notifiers: []
  prometheusAPIClientTLS: {}
  prometheusLongterm:
    diskSizeGigabytes: 40
    effectiveStorageClass: ceph-ssd
    retentionGigabytes: 32
  prometheusMain:
    diskSizeGigabytes: 35
    effectiveStorageClass: default
    retentionGigabytes: 28
  prometheusScraperIstioMTLS: {}
  prometheusScraperTLS: {}
  remoteWrite: []
  vpa:
    longtermMaxCPU: 2933m
    longtermMaxMemory: 2200Mi
    maxCPU: 8800m
    maxMemory: 6600Mi
longtermMaxDiskSizeGigabytes: 300
longtermRetentionDays: 0
longtermScrapeInterval: 5m
mainMaxDiskSizeGigabytes: 300
retentionDays: 15
scrapeInterval: 30s
`)
		f.HelmRender()
	})

	It("must not render SecurityPolicyException resources or exception pod labels", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		grafanaDeployment := f.KubernetesResource("Deployment", "d8-monitoring", "grafana-v10")
		Expect(grafanaDeployment.Exists()).To(BeTrue())
		Expect(grafanaDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())

		Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "grafana-v10").Exists()).To(BeFalse())
		Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "alerts-receiver").Exists()).To(BeFalse())
		Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "trickster").Exists()).To(BeFalse())
		Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "aggregating-proxy").Exists()).To(BeFalse())
	})

	It("if rendered, the prometheus-main prompptool init container must be restricted-PSS compliant", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		prometheusMain := f.KubernetesResource("Prometheus", "d8-monitoring", "main")
		if !prometheusMain.Exists() {
			Skip("Prometheus/main CR not rendered in this test setup")
		}

		initContainers := prometheusMain.Field("spec.initContainers").Array()
		if len(initContainers) == 0 {
			Skip("no initContainers rendered (prompp digests likely absent in test fixtures)")
		}

		found := false
		for _, c := range initContainers {
			if c.Get("name").String() != "prompptool" {
				continue
			}
			found = true
			drops := c.Get("securityContext.capabilities.drop").Array()
			dropStrings := make([]string, 0, len(drops))
			for _, d := range drops {
				dropStrings = append(dropStrings, d.String())
			}
			Expect(dropStrings).To(ContainElement("ALL"),
				"prompptool init container must drop ALL capabilities for d8-monitoring restricted PSS")
			Expect(c.Get("securityContext.allowPrivilegeEscalation").Bool()).To(BeFalse(),
				"prompptool init container must set allowPrivilegeEscalation:false")
			Expect(c.Get("securityContext.runAsNonRoot").Bool()).To(BeTrue(),
				"prompptool init container must runAsNonRoot:true")
			Expect(c.Get("securityContext.seccompProfile.type").String()).To(Equal("RuntimeDefault"),
				"prompptool init container must set seccompProfile.type=RuntimeDefault")
		}
		Expect(found).To(BeTrue(), "expected to find prompptool init container in Prometheus/main spec")
	})
})
