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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const providerID = "yandex"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

// fake *-crd modules are required for backward compatibility with lib_helm library
// TODO: remove fake crd modules
const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-yandex", "operator-prometheus", "operator-prometheus-crd", "prometheus", "prometheus-crd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: Yandex
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.32"
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
    kubernetesVersion: 1.32.0
    clusterUUID: 3b5058e1-e93a-4dfa-be32-395ef4b3da45
`

// The ModuleConfig v2 settings schema is extended into the module values schema,
// so provider and nodes are required here even though hooks fill them at runtime.
const moduleValues = `
  provider:
    parameters:
      cloudID: test
      folderID: myfoldid
  nodes:
    parameters:
      layout: WithNATInstance
      nodeNetworkCIDR: 10.100.0.1/24
      sshPublicKey: mysshkey
      labels:
        test: test
  ccm:
    parameters:
      additionalExternalNetworkIDs:
      - enp-external-1
      - enp-external-2
  storage:
    parameters: {}
  internal:
    credentialSecrets:
      d8-credentials:
        authScheme: ServiceAccount
        secret: '{"my": "json"}'
    instanceClassDefaults:
      imageID: test
    validationWebhookCert:
      crt: webhook-crt
      key: webhook-key
      ca: webhook-ca
    capyControllerManagerWebhookCert:
      crt: capy-crt
      key: capy-key
      ca: capy-ca
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
`

const tolerationsAnyNodeWithUninitialized = `
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: node.deckhouse.io/etcd-arbiter
- key: dedicated.deckhouse.io
  operator: "Exists"
- key: dedicated
  operator: "Exists"
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
- effect: NoSchedule
  key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: ToBeDeletedTaint
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/csi-not-bootstrapped
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable`

const moduleNamespace = "d8-cloud-provider-yandex"

// envValue pulls one environment variable out of the first container of a workload.
func envValue(resource object_store.KubeObject, name string) (string, bool) {
	for _, env := range resource.Field("spec.template.spec.containers.0.env").Array() {
		if env.Get("name").String() == name {
			return env.Get("value").String(), true
		}
	}
	return "", false
}

// modulesImagesWithout copies the shared image digests and drops one image of this module,
// so the "image is not built in this edition" branch can be exercised. GetModulesImages
// hands out the package-level library.DefaultImagesDigests map, which must not be mutated:
// every other spec in the suite reads it too.
func modulesImagesWithout(imageName string) map[string]interface{} {
	images := GetModulesImages()

	digests := map[string]interface{}{}
	for module, moduleImages := range images["digests"].(map[string]interface{}) {
		if module != "cloudProviderYandex" {
			digests[module] = moduleImages
			continue
		}

		kept := map[string]interface{}{}
		for name, digest := range moduleImages.(map[string]interface{}) {
			if name == imageName {
				continue
			}
			kept[name] = digest
		}
		digests[module] = kept
	}

	images["digests"] = digests

	return images
}

// registrationYandexValues decodes the provider-specific blob of a registration Secret.
func registrationYandexValues(resource object_store.KubeObject) string {
	decoded, err := base64.StdEncoding.DecodeString(resource.Field("data.yandex").String())
	Expect(err).ShouldNot(HaveOccurred())
	return string(decoded)
}

var _ = Describe("Module :: cloud-provider-yandex :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("cloudProviderYandex", moduleValues)
	})

	Context("Yandex exporter", func() {
		assertExporterDeploymentSecret := func(h *Config, exists bool) {
			deployment := h.KubernetesResource("Deployment", moduleNamespace, "cloud-metrics-exporter")
			Expect(deployment.Exists()).To(Equal(exists))

			secret := h.KubernetesResource("Secret", moduleNamespace, "cloud-metrics-exporter-app-creds")
			Expect(secret.Exists()).To(Equal(exists))
			if exists {
				Expect(secret.Field("data.api-key").String()).To(Equal("YXBpLWtleQ=="))
				Expect(secret.Field("data.folder-id").String()).To(Equal("bXlmb2xkaWQ="))
			}

			pdb := h.KubernetesResource("PodDisruptionBudget", moduleNamespace, "cloud-metrics-exporter")
			Expect(pdb.Exists()).To(Equal(exists))
		}

		assertDeployNatInstanceMonitoring := func(h *Config, exists bool) {
			prometheusRuleExists := h.KubernetesResource("PrometheusRule", moduleNamespace, "cloud-provider-yandex-nat-instance").Exists()
			grafanaDashboardExists := h.KubernetesResource("GrafanaDashboardDefinition", "", "d8-cloud-provider-yandex-kubernetes-cluster-nat-instance").Exists()
			monitor := h.KubernetesResource("PodMonitor", "d8-monitoring", "yandex-nat-instance-metrics")

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

				It("Should keep the exporter VPA while vertical-pod-autoscaler is enabled", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					Expect(f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-metrics-exporter").Exists()).To(BeTrue())
				})
			})

			Context("with NAT-instance and vertical-pod-autoscaler disabled", func() {
				BeforeEach(func() {
					f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.natInstanceName", "cluster-nat-instance")
					f.ValuesSetFromYaml("global.enabledModules", `["cloud-provider-yandex", "operator-prometheus", "operator-prometheus-crd"]`)
					f.HelmRender()
				})

				It("Should drop the VPA and inline the resource requests instead", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					Expect(f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-metrics-exporter").Exists()).To(BeFalse())

					deployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-metrics-exporter")
					Expect(deployment.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("10m"))
					Expect(deployment.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("25Mi"))
				})
			})

			Context("with NAT-instance and operator-prometheus disabled", func() {
				BeforeEach(func() {
					f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.natInstanceName", "cluster-nat-instance")
					f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-yandex"]`)
					f.HelmRender()
				})

				It("Should not create the PodMonitor", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "yandex-nat-instance-metrics").Exists()).To(BeFalse())
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

			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			registrySecret := f.KubernetesResource("Secret", moduleNamespace, "deckhouse-registry")

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "yandex.csi.flant.com")
			csiControllerSS := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			csiNodeDS := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			csiControllerSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:csi:controller:external-provisioner")
			csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:csi:controller:external-provisioner")
			csiExternalAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:csi:controller:external-attacher")
			csiExternalAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:csi:controller:external-attacher")
			csiExternalResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:csi:controller:external-resizer")
			csiExternalResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:csi:controller:external-resizer")
			csiCredentials := f.KubernetesResource("Secret", moduleNamespace, "csi-credentials")
			csiHDDSC := f.KubernetesGlobalResource("StorageClass", "network-hdd")
			csiSSDSC := f.KubernetesGlobalResource("StorageClass", "network-ssd")
			csiSSDSCNonReplicated := f.KubernetesGlobalResource("StorageClass", "network-ssd-nonreplicated")

			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-yandex:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-yandex:cluster-admin")

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
			Expect(userAuthzUser.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - yandexinstanceclasses
  verbs:
  - get
  - list
  - watch`))
			Expect(userAuthzClusterAdmin.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - yandexinstanceclasses
  verbs:
  - create
  - delete
  - deletecollection
  - patch
  - update`))

			// user story #1
			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

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

			Expect(registrationYandexValues(providerRegistrationSecret)).To(MatchJSON(expectedProviderRegistrationJSON))
			Expect(registrationYandexValues(providerSpecificRegistrationSecret)).To(MatchJSON(expectedProviderRegistrationJSON))

			providerSpecificMCMSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-mcm", providerID))
			Expect(providerSpecificMCMSecret.Exists()).To(BeTrue())
			Expect(providerSpecificMCMSecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("mcm"))
			Expect(providerSpecificMCMSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificMCMSecretData := providerSpecificMCMSecret.Field("data").Map()
			Expect(providerSpecificMCMSecretData).To(Not(BeEmpty()))
			Expect(len(providerSpecificMCMSecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificMCMSecretData["config-for-machine-controller-manager.yaml"].String()) > 0).To(BeTrue())

			providerSpecificCAPISecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-capi", providerID))
			Expect(providerSpecificCAPISecret.Exists()).To(BeTrue())
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("capi"))

			providerSpecificBashibleStepsSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-steps", providerID))
			Expect(providerSpecificBashibleStepsSecret.Exists()).To(BeTrue())
			Expect(providerSpecificBashibleStepsSecret.Field(fmt.Sprintf("metadata.labels.%s", bashibleLabelKey)).String()).To(Equal("steps"))
			Expect(providerSpecificBashibleStepsSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificBashibleStepsSecretData := providerSpecificBashibleStepsSecret.Field("data").Map()
			Expect(len(providerSpecificBashibleStepsSecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificBashibleStepsSecretData["000_set_cloud_variables.sh.tpl"].String()) > 0).To(BeTrue())

			providerSpecificBashibleBootstrapSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-bootstrap", providerID))
			Expect(providerSpecificBashibleBootstrapSecret.Exists()).To(BeTrue())
			Expect(providerSpecificBashibleBootstrapSecret.Field(fmt.Sprintf("metadata.labels.%s", bashibleLabelKey)).String()).To(Equal("bootstrap"))
			Expect(providerSpecificBashibleBootstrapSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificBashibleBootstrapSecretData := providerSpecificBashibleBootstrapSecret.Field("data").Map()
			Expect(len(providerSpecificBashibleBootstrapSecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificBashibleBootstrapSecretData["bootstrap-networks.sh.tpl"].String()) > 0).To(BeTrue())

			// user story #2
			Expect(csiDriver.Exists()).To(BeTrue())
			Expect(csiControllerSS.Exists()).To(BeTrue())
			Expect(csiControllerSS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(csiNodeDS.Exists()).To(BeTrue())
			Expect(csiNodeDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
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

			// The credentials of every workload come from the credential Secret, not from
			// the retired providerClusterConfiguration.
			Expect(csiCredentials.Field("data.serviceAccountJSON").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(`{"my": "json"}`))))
			Expect(ccmSecret.Field("data.service-acount-json").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(`{"my": "json"}`))))
			Expect(f.KubernetesResource("Secret", moduleNamespace, "yandex-credentials-capy").Field("data.key").String()).
				To(Equal(base64.StdEncoding.EncodeToString([]byte(`{"my": "json"}`))))

			Expect(csiHDDSC.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))

			Expect(ccmSA.Exists()).To(BeTrue())
			Expect(ccmCR.Exists()).To(BeTrue())
			Expect(ccmCRB.Exists()).To(BeTrue())
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmDeploy.Exists()).To(BeTrue())
			Expect(ccmSecret.Exists()).To(BeTrue())

			// The folder ID and the external network IDs come from ModuleConfig v2 settings,
			// not from the v1 root paths.
			folderID, found := envValue(ccmDeploy, "YANDEX_CLOUD_FOLDER_ID")
			Expect(found).To(BeTrue())
			Expect(folderID).To(Equal("myfoldid"))

			externalNetworkIDs, found := envValue(ccmDeploy, "YANDEX_CLOUD_EXTERNAL_NETWORK_IDS")
			Expect(found).To(BeTrue())
			Expect(externalNetworkIDs).To(Equal("enp-external-1,enp-external-2"))

			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))

			cddFolderID, found := envValue(cddDeployment, "YC_FOLDER_ID")
			Expect(found).To(BeTrue())
			Expect(cddFolderID).To(Equal("myfoldid"))
		})
	})

	Context("Registration Secret", func() {
		Context("with an empty sshPublicKey", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.nodes.parameters", "\nlayout: WithNATInstance\nnodeNetworkCIDR: 10.100.0.1/24\nsshPublicKey: \"\"\nlabels:\n  test: test\n")
				f.HelmRender()
			})

			It("omits both the top-level key and the provider-specific one", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
				Expect(secret.Field("data.sshPublicKey").Exists()).To(BeFalse())
				Expect(registrationYandexValues(secret)).ShouldNot(ContainSubstring("sshKey"))
			})
		})

		Context("without a credential Secret", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.internal.credentialSecrets", `{}`)
				f.HelmRender()
			})

			It("omits serviceAccountJSON and renders empty credentials for the workloads", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
				Expect(registrationYandexValues(secret)).ShouldNot(ContainSubstring("serviceAccountJSON"))

				Expect(f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager").Field("data.service-acount-json").String()).To(Equal(""))
				Expect(f.KubernetesResource("Secret", moduleNamespace, "csi-credentials").Field("data.serviceAccountJSON").String()).To(Equal(""))
			})
		})

		Context("without node labels", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.nodes.parameters", `
layout: WithNATInstance
nodeNetworkCIDR: 10.100.0.1/24
sshPublicKey: mysshkey
labels: {}
`)
				f.HelmRender()
			})

			It("falls back to an empty label map", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
				Expect(registrationYandexValues(secret)).To(ContainSubstring(`"labels":{}`))
			})
		})

		Context("without an instance class default image", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.internal.instanceClassDefaults", `{}`)
				f.HelmRender()
			})

			It("still emits the key so node-manager can dereference it", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
				Expect(registrationYandexValues(secret)).To(ContainSubstring(`"instanceClassDefaults":{"imageID":""}`))
			})
		})
	})

	Context("CNI Secret", func() {
		Context("without cniSecretData", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("defaults to cilium in VXLAN mode", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
				Expect(secret.Exists()).To(BeTrue())

				cni, err := base64.StdEncoding.DecodeString(secret.Field("data.cni").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(cni)).To(Equal("cilium"))

				cilium, err := base64.StdEncoding.DecodeString(secret.Field("data.cilium").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(cilium)).To(MatchJSON(`{"mode": "VXLAN", "masqueradeMode": "BPF"}`))
			})
		})

		Context("with cniSecretData", func() {
			BeforeEach(func() {
				// "cni: simple-bridge"
				f.ValuesSet("cloudProviderYandex.internal.cniSecretData", base64.StdEncoding.EncodeToString([]byte("cni: c2ltcGxlLWJyaWRnZQ==")))
				f.HelmRender()
			})

			It("uses the stored data verbatim", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
				cni, err := base64.StdEncoding.DecodeString(secret.Field("data.cni").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(cni)).To(Equal("simple-bridge"))
				Expect(secret.Field("data.cilium").Exists()).To(BeFalse())
			})
		})
	})

	Context("Disabled sections", func() {
		Context("nodes.disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.nodes.disabled", true)
				f.HelmRender()
			})

			It("drops the CCM and the CAPI controller but keeps storage", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "capy-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", moduleNamespace, "yandex-credentials-capy").Exists()).To(BeFalse())

				// cloud-data-discoverer reads this Secret, so it survives a disabled CCM.
				Expect(f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager").Exists()).To(BeTrue())

				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("StorageClass", "network-hdd").Exists()).To(BeTrue())
			})
		})

		Context("ccm.disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.ccm.disabled", true)
				f.HelmRender()
			})

			It("drops the CCM Deployment but keeps the CAPI controller and storage", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "capy-controller-manager").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeTrue())
			})
		})

		Context("storage.disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.storage.disabled", true)
				f.HelmRender()
			})

			It("drops the CSI stack but keeps the CCM", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesGlobalResource("CSIDriver", "yandex.csi.flant.com").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", moduleNamespace, "csi-credentials").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, "csi").Exists()).To(BeFalse())
				Expect(f.KubernetesGlobalResource("StorageClass", "network-hdd").Exists()).To(BeFalse())

				Expect(f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager").Exists()).To(BeTrue())
			})
		})

		Context("nodes.disabled and storage.disabled together", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.nodes.disabled", true)
				f.ValuesSet("cloudProviderYandex.storage.disabled", true)
				f.HelmRender()
			})

			It("still renders the namespace, the registration Secrets and the webhook", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesGlobalResource("Namespace", moduleNamespace).Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "validation-webhook").Exists()).To(BeTrue())
			})
		})
	})

	Context("Storage classes", func() {
		Context("with an empty storage class list", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.internal.storageClasses", `[]`)
				f.HelmRender()
			})

			It("renders no StorageClass at all", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesGlobalResource("StorageClass", "network-hdd").Exists()).To(BeFalse())
				Expect(f.KubernetesGlobalResource("StorageClass", "network-ssd").Exists()).To(BeFalse())
			})
		})
	})

	Context("Validation webhook", func() {
		Context("in a bootstrapped cluster", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("renders the whole serving stack off the generated certificate", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "validation-webhook")
				Expect(deployment.Exists()).To(BeTrue())
				Expect(deployment.Field("spec.template.spec.hostNetwork").Exists()).To(BeFalse())
				Expect(deployment.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).Exists()).To(BeFalse())

				Expect(f.KubernetesResource("Service", moduleNamespace, "validation-webhook").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, "validation-webhook").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("PodDisruptionBudget", moduleNamespace, "validation-webhook").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "validation-webhook").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:validation-webhook").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:validation-webhook").Exists()).To(BeTrue())

				tlsSecret := f.KubernetesResource("Secret", moduleNamespace, "validation-webhook-tls")
				Expect(tlsSecret.Exists()).To(BeTrue())
				Expect(tlsSecret.Field("data.tls\\.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("webhook-crt"))))
				Expect(tlsSecret.Field("data.tls\\.key").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("webhook-key"))))
				Expect(tlsSecret.Field("data.ca\\.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("webhook-ca"))))

				webhookConfiguration := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-cloud-provider-yandex-validation-webhook")
				Expect(webhookConfiguration.Exists()).To(BeTrue())

				webhookNames := []string{}
				for _, webhook := range webhookConfiguration.Field("webhooks").Array() {
					webhookNames = append(webhookNames, webhook.Get("name").String())
					Expect(webhook.Get("clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("webhook-ca"))))
				}
				Expect(webhookNames).To(ConsistOf(
					"moduleconfigs.cloud-provider-yandex.deckhouse.io",
					"secrets.cloud-provider-yandex.deckhouse.io",
					"nodegroups.cloud-provider-yandex.deckhouse.io",
					"yandexinstanceclasses.cloud-provider-yandex.deckhouse.io-v1alpha1",
					"yandexinstanceclasses.cloud-provider-yandex.deckhouse.io-v1",
				))
			})
		})

		Context("in a cluster that is not bootstrapped yet", func() {
			BeforeEach(func() {
				f.ValuesSet("global.clusterIsBootstrapped", false)
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-yandex", "admission-policy-engine-crd"]`)
				f.HelmRender()
			})

			It("runs on the host network and carries the security policy exception", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "validation-webhook")
				Expect(deployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
				Expect(deployment.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()).To(Equal("validation-webhook"))

				exception := f.KubernetesResource("SecurityPolicyException", moduleNamespace, "validation-webhook")
				Expect(exception.Exists()).To(BeTrue())
				Expect(exception.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			})
		})

		Context("without admission-policy-engine-crd", func() {
			BeforeEach(func() {
				f.ValuesSet("global.clusterIsBootstrapped", false)
				f.HelmRender()
			})

			It("skips the security policy exception", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("SecurityPolicyException", moduleNamespace, "validation-webhook").Exists()).To(BeFalse())
			})
		})

		Context("without vertical-pod-autoscaler-crd", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["cloud-provider-yandex"]`)
				f.HelmRender()
			})

			It("drops the VPA and inlines the resource requests instead", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "validation-webhook").Exists()).To(BeFalse())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "validation-webhook")
				Expect(deployment.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("25m"))
				Expect(deployment.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("64Mi"))
			})
		})
	})

	Context("Namespace", func() {
		Context("with admission-policy-engine-crd enabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-yandex", "admission-policy-engine-crd"]`)
				f.HelmRender()
			})

			It("asks for the security policy check", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
				Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/enable-security-policy-check`).String()).To(Equal("true"))
			})
		})

		Context("with admission-policy-engine-crd disabled", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("omits the security policy check label", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
				Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/enable-security-policy-check`).Exists()).To(BeFalse())
			})
		})
	})

	Context("CAPI controller", func() {
		Context("in a bootstrapped cluster", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("relies on the in-cluster apiserver endpoint", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "capy-controller-manager")
				Expect(deployment.Exists()).To(BeTrue())

				_, found := envValue(deployment, "KUBERNETES_SERVICE_HOST")
				Expect(found).To(BeFalse())
			})
		})

		Context("in a cluster that is not bootstrapped yet", func() {
			BeforeEach(func() {
				f.ValuesSet("global.clusterIsBootstrapped", false)
				f.HelmRender()
			})

			It("points the controller at the host apiserver", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "capy-controller-manager")
				port, found := envValue(deployment, "KUBERNETES_SERVICE_PORT")
				Expect(found).To(BeTrue())
				Expect(port).To(Equal("6443"))
			})
		})
	})

	Context("Credential Secret lookup", func() {
		Context("when the Secret carries no secret field", func() {
			BeforeEach(func() {
				// authScheme alone is a valid Secret shape: an identity-only scheme stores
				// no secret, and the workloads must still render.
				f.ValuesSetFromYaml("cloudProviderYandex.internal.credentialSecrets", `
d8-credentials:
  authScheme: ServiceAccount
  secret: ""
  identity: my-identity
`)
				f.HelmRender()
			})

			It("renders an empty credential instead of failing", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager").Field("data.service-acount-json").String()).To(Equal(""))
				Expect(f.KubernetesResource("Secret", moduleNamespace, "csi-credentials").Field("data.serviceAccountJSON").String()).To(Equal(""))
				Expect(f.KubernetesResource("Secret", moduleNamespace, "yandex-credentials-capy").Field("data.key").String()).To(Equal(""))
			})
		})

		Context("when a differently named Secret is present", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.internal.credentialSecrets", `
d8-credentials-exporter:
  authScheme: APIToken
  secret: exporter-key
`)
				f.HelmRender()
			})

			It("does not pick it up for the provider credentials", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager").Field("data.service-acount-json").String()).To(Equal(""))
			})
		})
	})

	Context("CSI controller image availability", func() {
		Context("when the CSI plugin image is missing from the release", func() {
			BeforeEach(func() {
				f.ValuesSet("global.modulesImages", modulesImagesWithout("yandexCsiPlugin"))
				f.HelmRender()
			})

			It("renders neither the controller nor the node DaemonSet", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node").Exists()).To(BeFalse())

				// The CSIDriver and the StorageClasses do not depend on the image.
				Expect(f.KubernetesGlobalResource("CSIDriver", "yandex.csi.flant.com").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("StorageClass", "network-hdd").Exists()).To(BeTrue())
			})
		})
	})

	Context("Exporter RBAC", func() {
		Context("with a monitoring api-key", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.monitoringAPIKey", "api-key")
				f.HelmRender()
			})

			It("renders the exporter ServiceAccount and its rbac-proxy bindings", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-metrics-exporter").Exists()).To(BeTrue())
			})
		})

		Context("without a monitoring api-key", func() {
			BeforeEach(func() {
				f.ValuesSet("cloudProviderYandex.internal.providerDiscoveryData.monitoringAPIKey", "")
				f.HelmRender()
			})

			It("renders no exporter RBAC at all", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-metrics-exporter").Exists()).To(BeFalse())
			})
		})
	})

	Context("CAPI controller webhook certificate", func() {
		Context("with a generated certificate", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("renders the TLS Secret from the values", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", moduleNamespace, "capy-controller-manager-webhook-tls")
				Expect(secret.Exists()).To(BeTrue())
				Expect(secret.Field("data.tls\\.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("capy-crt"))))
			})
		})

		Context("before the certificate hook has run", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderYandex.internal", `
credentialSecrets:
  d8-credentials:
    authScheme: ServiceAccount
    secret: '{"my": "json"}'
instanceClassDefaults:
  imageID: test
validationWebhookCert:
  crt: webhook-crt
  key: webhook-key
  ca: webhook-ca
storageClasses: []
providerDiscoveryData:
  apiVersion: deckhouse.io/v1
  kind: YandexCloudDiscoveryData
  zones: ["zonea"]
  zoneToSubnetIdMap:
    zonea: aaa
  defaultLbTargetGroupNetworkId: deftarggroupnetid
  internalNetworkIDs: ["id1"]
  shouldAssignPublicIPAddress: true
  routeTableID: testest
  region: myreg
  natInstanceName: ""
`)
				f.HelmRender()
			})

			It("skips the TLS Secret instead of failing the render", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				Expect(f.KubernetesResource("Secret", moduleNamespace, "capy-controller-manager-webhook-tls").Exists()).To(BeFalse())
			})
		})
	})

	Context("CAPI controller security policy", func() {
		Context("with admission-policy-engine-crd enabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-yandex", "admission-policy-engine-crd"]`)
				f.HelmRender()
			})

			It("labels the pod for the security policy exception", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "capy-controller-manager")
				Expect(deployment.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()).To(Equal("capy-controller-manager"))
			})
		})

		Context("with admission-policy-engine-crd disabled", func() {
			BeforeEach(func() {
				f.HelmRender()
			})

			It("omits the label", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", moduleNamespace, "capy-controller-manager")
				Expect(deployment.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).Exists()).To(BeFalse())
			})
		})
	})

	Context("Yandex with discovered default StorageClass (without `global.defaultClusterStorageClass`)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.discovery.defaultStorageClass", `network-ssd`)
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
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Yandex with discovered default StorageClass AND `global.defaultClusterStorageClass` specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.discovery.defaultStorageClass", `network-ssd`)
			f.ValuesSetFromYaml("global.defaultClusterStorageClass", `network-ssd`)
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
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Yandex bootstraped cluster (no default StorageClass yet)", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly with proper (first in list) default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			csiHDDSC := f.KubernetesGlobalResource("StorageClass", "network-hdd")
			csiSSDSC := f.KubernetesGlobalResource("StorageClass", "network-ssd")
			csiSSDSCNonReplicated := f.KubernetesGlobalResource("StorageClass", "network-ssd-nonreplicated")

			Expect(csiHDDSC.Exists()).To(BeTrue())
			Expect(csiSSDSC.Exists()).To(BeTrue())
			Expect(csiSSDSCNonReplicated.Exists()).To(BeTrue())

			Expect(csiHDDSC.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
			Expect(csiSSDSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(csiSSDSCNonReplicated.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
		})
	})

})
