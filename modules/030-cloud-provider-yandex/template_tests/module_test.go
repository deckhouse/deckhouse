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

/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, yandex-csi.

*/

package template_tests

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd", "cloud-provider-yandex", "operator-prometheus-crd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: Yandex
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.29"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.29.0
    clusterUUID: 3b5058e1-e93a-4dfa-be32-395ef4b3da45
`

const moduleValues = `
  internal:
    storageClasses:
    - name: network-hdd
      type: network-hdd
    - name: network-ssd
      type: network-ssd
    - name: network-ssd-nonreplicated
      type: network-ssd-nonreplicated
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: YandexCloudDiscoveryData
      zones: ["zonea", "zoneb"]
      zoneToSubnetIdMap:
        zonea: aaa
        zoneb: bbb
      defaultLbTargetGroupNetworkId: deftarggroupnetid
      internalNetworkIDs: ["id1", "id2"]
      shouldAssignPublicIPAddress: true
      routeTableID: testest
      region: myreg
      natInstanceName: ""
    providerClusterConfiguration:
      apiVersion: deckhouse.io/v1
      existingNetworkID: enpma5uvcfbkuac1i1jb
      kind: YandexClusterConfiguration
      layout: WithNATInstance
      masterNodeGroup:
        replicas: 1
        instanceClass:
          cores: 2
          imageID: test
          memory: 4096
      provider:
        cloudID: test
        folderID: myfoldid
        serviceAccountJSON: '{"my": "json"}'
      withNATInstance:
        internalSubnetID: test
        natInstanceExternalAddress: 84.201.160.148
      nodeNetworkCIDR: 10.100.0.1/24
      sshPublicKey: mysshkey
      labels:
        test: test
`

var _ = Describe("Module :: cloud-provider-yandex :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("cloudProviderYandex", moduleValues)
	})

	Context("Yandex exporter", func() {
		assertExporterDeploymentSecret := func(h *Config, exists bool) {
			deployment := h.KubernetesResource("Deployment", "d8-cloud-provider-yandex", "cloud-metrics-exporter")
			Expect(deployment.Exists()).To(Equal(exists))

			secret := h.KubernetesResource("Secret", "d8-cloud-provider-yandex", "cloud-metrics-exporter-app-creds")
			Expect(secret.Exists()).To(Equal(exists))
			if exists {
				Expect(secret.Field("data.api-key").String()).To(Equal("YXBpLWtleQ=="))
				Expect(secret.Field("data.folder-id").String()).To(Equal("bXlmb2xkaWQ="))
			}
		}

		assertDeployNatInstanceMonitoring := func(h *Config, exists bool) {
			prometheusRuleExists := h.KubernetesResource("PrometheusRule", "d8-cloud-provider-yandex", "cloud-provider-yandex-nat-instance").Exists()
			grafanaDashboardExists := h.KubernetesResource("GrafanaDashboardDefinition", "", "d8-cloud-provider-yandex-kubernetes-cluster-nat-instance").Exists()
			monitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "yandex-nat-instance-metrics")

			Expect(monitor.Exists()).To(Equal(exists))
			Expect(prometheusRuleExists).To(BeTrue())
			Expect(grafanaDashboardExists).To(Equal(exists))
		}

		Context("monitoring api-key does not set", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.monitoringAPIKey", "")
			})

			Context("without NAT-instance", func() {
				BeforeEach(func() {
					f.HelmRender()
				})

				It("Should not create deployment with exporter and secret with creds for exporter", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertExporterDeploymentSecret(f, false)
				})

				It("Should not deploy monitor, prometheus rules and grafana dashboard for nat instance", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertDeployNatInstanceMonitoring(f, false)
				})
			})

			Context("with NAT-instance", func() {
				BeforeEach(func() {
					f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.natInstanceName", "cluster-nat-instance")
					f.HelmRender()
				})

				It("Should not create deployment with exporter and secret with creds for exporter", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertExporterDeploymentSecret(f, false)
				})

				It("Should not deploy monitor, prometheus rules and grafana dashboard for nat instance", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertDeployNatInstanceMonitoring(f, false)
				})
			})
		})

		Context("monitoring api-key sets", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.monitoringAPIKey", "api-key")
			})

			Context("without NAT-instance", func() {
				BeforeEach(func() {
					f.HelmRender()
				})

				It("Should create deployment with exporter and secret with creds for exporter", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertExporterDeploymentSecret(f, true)
				})

				It("Should not deploy monitor, prometheus rules and grafana dashboard for nat instance", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertDeployNatInstanceMonitoring(f, false)
				})
			})

			Context("with NAT-instance", func() {
				BeforeEach(func() {
					f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.natInstanceName", "cluster-nat-instance")
					f.HelmRender()
				})

				It("Should create deployment with exporter and secret with creds for exporter", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertExporterDeploymentSecret(f, true)
				})

				It("Should deploy monitor, prometheus rules and grafana dashboard for nat instance", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertDeployNatInstanceMonitoring(f, true)
				})
			})
		})
	})

	Context("Yandex", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-yandex")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "yandex.csi.flant.com")
			csiControllerSS := f.KubernetesResource("Deployment", "d8-cloud-provider-yandex", "csi-controller")
			csiNodeDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-yandex", "csi-node")
			csiControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-yandex", "csi")
			csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:csi:controller:external-provisioner")
			csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:csi:controller:external-provisioner")
			csiExternalAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:csi:controller:external-attacher")
			csiExternalAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:csi:controller:external-attacher")
			csiExternalResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:csi:controller:external-resizer")
			csiExternalResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:csi:controller:external-resizer")
			csiCredentials := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "csi-credentials")
			csiHDDSC := f.KubernetesGlobalResource("StorageClass", "network-hdd")
			csiSSDSC := f.KubernetesGlobalResource("StorageClass", "network-ssd")
			csiSSDSCNonReplicated := f.KubernetesGlobalResource("StorageClass", "network-ssd-nonreplicated")

			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-yandex", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-yandex", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-yandex", "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-yandex:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-yandex:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "folderID": "myfoldid",
          "region": "myreg",
          "serviceAccountJSON": "{\"my\": \"json\"}",
          "sshKey": "mysshkey",
          "zoneToSubnetIdMap": {
            "zonea": "aaa",
            "zoneb": "bbb"
          },
          "shouldAssignPublicIPAddress": true,
          "labels": {"test": "test"},
		  "nodeNetworkCIDR": "10.100.0.1/24",
		  "instanceClassDefaults": {
			  "imageID": "test"
		  }
        }`
			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.yandex").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			// user story #2
			Expect(csiDriver.Exists()).To(BeTrue())
			Expect(csiControllerSS.Exists()).To(BeTrue())
			Expect(csiNodeDS.Exists()).To(BeTrue())
			Expect(csiControllerSA.Exists()).To(BeTrue())
			Expect(csiProvisionerCR.Exists()).To(BeTrue())
			Expect(csiProvisionerCRB.Exists()).To(BeTrue())
			Expect(csiExternalAttacherCR.Exists()).To(BeTrue())
			Expect(csiExternalAttacherCRB.Exists()).To(BeTrue())
			Expect(csiExternalResizerCR.Exists()).To(BeTrue())
			Expect(csiExternalResizerCRB.Exists()).To(BeTrue())
			Expect(csiCredentials.Exists()).To(BeTrue())
			Expect(csiHDDSC.Exists()).To(BeTrue())
			Expect(csiSSDSC.Exists()).To(BeTrue())
			Expect(csiSSDSCNonReplicated.Exists()).To(BeTrue())

			Expect(csiHDDSC.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.deckhouse.io/volume-expansion-mode: offline
storageclass.kubernetes.io/is-default-class: "true"
`))

			Expect(ccmSA.Exists()).To(BeTrue())
			Expect(ccmCR.Exists()).To(BeTrue())
			Expect(ccmCRB.Exists()).To(BeTrue())
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmDeploy.Exists()).To(BeTrue())
			Expect(ccmSecret.Exists()).To(BeTrue())
		})

		Context("Unsupported Kubernetes version", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderYandex", moduleValues)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CCM and CSI controller should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-yandex", "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-yandex", "csi-controller").Exists()).To(BeFalse())
			})
		})
	})

	Context("Yabdex with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderYandex", moduleValues)
			f.ValuesSetFromYaml("cloudProviderYandex.internal.defaultStorageClass", `network-ssd`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			csiHDDSC := f.KubernetesGlobalResource("StorageClass", "network-hdd")
			csiSSDSC := f.KubernetesGlobalResource("StorageClass", "network-ssd")
			csiSSDSCNonReplicated := f.KubernetesGlobalResource("StorageClass", "network-ssd-nonreplicated")

			Expect(csiHDDSC.Exists()).To(BeTrue())
			Expect(csiSSDSC.Exists()).To(BeTrue())
			Expect(csiSSDSCNonReplicated.Exists()).To(BeTrue())

			Expect(csiHDDSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(csiSSDSC.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.deckhouse.io/volume-expansion-mode: offline
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

})
