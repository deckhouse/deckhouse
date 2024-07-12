/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, cinder-csi-driver.

*/

package template_tests

import (
	"encoding/base64"
	"fmt"
	"os"
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

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: OpenStack
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "%s"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      master: 3
      worker: 1
    podSubnet: 10.0.1.0/16
    kubernetesVersion: "%s.4"
    defaultStorageClass: fastssd
`

const moduleValues = `
  storageClass:
    topologyEnabled: true
  ignoreVolumeMicroversion: false
  internal:
    storageClasses:
      - name: fastssd
        type: Fast HDD
      - name: slowhdd
        type: Slow HDD
    connection:
      authURL: http://my.cloud.lalla/123/
      username: myuser
      password: myPaSs
      domainName: mydomain
      tenantName: mytenantname
      caCert: mycacert
      region: myreg
    internalNetworkNames:
      - myintnetname
      - myintnetname2
    externalNetworkNames:
      - myextnetname
      - myextnetname2
    podNetworkMode: "VXLAN"
    instances:
      imageName: ubuntu
      mainNetwork: kube
      securityGroups: ["aaa","bbb"]
      sshKeyPairName: mysshkeypairname
    zones: ["zonea", "zoneb"]
    loadBalancer:
      subnetID: my-subnet-id
      floatingNetworkID: my-floating-network-id
`

const badModuleValues = `
  internal:
    connection:
      authURL: http://my.cloud.lalla/123/
      username: myuser
      password: myPaSs
      domainName: mydomain
      tenantName: mytenantname
      tenantID: mytenantid
      caCert: mycacert
      region: myreg
    internalNetworkNames:
      - myintnetname
      - myintnetname2
    externalNetworkNames:
      - myextnetname
      - myextnetname2
    podNetworkMode: "VXLAN"
    instances:
      sshKeyPairName: mysshkeypairname
      securityGroups: ["aaa","bbb"]
    zones: ["zonea", "zoneb"]
`

func openstackCheck(f *Config, k8sVer string) {
	BeforeEach(func() {
		f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, k8sVer, k8sVer))
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
		f.HelmRender()
	})

	It("Everything must render properly", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-openstack")
		registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "deckhouse-registry")

		providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

		cinderControllerPluginSS := f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "csi-controller")
		cinderCSIDriver := f.KubernetesGlobalResource("CSIDriver", "cinder.csi.openstack.org")
		cinderNodePluginDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-openstack", "csi-node")
		cinderControllerPluginSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "csi")
		cinderProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:csi:controller:external-provisioner")
		cinderProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:csi:controller:external-provisioner")
		cinderAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:csi:controller:external-attacher")
		cinderAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:csi:controller:external-attacher")
		cinderResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:csi:controller:external-resizer")
		cinderResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:csi:controller:external-resizer")

		ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cloud-controller-manager")
		ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cloud-controller-manager")
		ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cloud-controller-manager")
		ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-openstack", "cloud-controller-manager")
		ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "cloud-controller-manager")
		ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "cloud-controller-manager")

		userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-openstack:user")
		userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-openstack:cluster-admin")

		scFast := f.KubernetesGlobalResource("StorageClass", "fastssd")
		scSlow := f.KubernetesGlobalResource("StorageClass", "slowhdd")

		Expect(namespace.Exists()).To(BeTrue())
		Expect(registrySecret.Exists()).To(BeTrue())

		// user story #1
		Expect(providerRegistrationSecret.Exists()).To(BeTrue())
		expectedProviderRegistrationJSON := `{
          "connection": {
            "authURL": "http://my.cloud.lalla/123/",
            "caCert": "mycacert",
            "domainName": "mydomain",
            "region": "myreg",
            "password": "myPaSs",
            "tenantName": "mytenantname",
            "username": "myuser"
          },
          "internalNetworkNames": ["myintnetname", "myintnetname2"],
          "externalNetworkNames": ["myextnetname", "myextnetname2"],
          "instances": {
            "imageName": "ubuntu",
            "mainNetwork": "kube",
            "securityGroups": [
              "aaa",
              "bbb"
            ],
            "sshKeyPairName": "mysshkeypairname"
          },
          "podNetworkMode": "VXLAN"
        }`
		providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.openstack").String())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

		// user story #2
		Expect(cinderCSIDriver.Exists()).To(BeTrue())
		Expect(cinderNodePluginDS.Exists()).To(BeTrue())
		Expect(cinderControllerPluginSA.Exists()).To(BeTrue())
		Expect(cinderControllerPluginSS.Exists()).To(BeTrue())
		Expect(cinderControllerPluginSS.Field("spec.template.spec.containers.0.args.3").String()).To(MatchYAML(`--feature-gates=Topology=true`))
		Expect(cinderAttacherCR.Exists()).To(BeTrue())
		Expect(cinderAttacherCRB.Exists()).To(BeTrue())
		Expect(cinderProvisionerCR.Exists()).To(BeTrue())
		Expect(cinderProvisionerCRB.Exists()).To(BeTrue())
		Expect(cinderResizerCR.Exists()).To(BeTrue())
		Expect(cinderResizerCRB.Exists()).To(BeTrue())
		Expect(cinderResizerCR.Exists()).To(BeTrue())
		Expect(cinderResizerCRB.Exists()).To(BeTrue())

		Expect(ccmSA.Exists()).To(BeTrue())
		Expect(ccmCR.Exists()).To(BeTrue())
		Expect(ccmCRB.Exists()).To(BeTrue())
		Expect(ccmVPA.Exists()).To(BeTrue())
		Expect(ccmDeploy.Exists()).To(BeTrue())
		Expect(ccmSecret.Exists()).To(BeTrue())
		ccmExpectedConfig := `
[Global]
auth-url = "http://my.cloud.lalla/123/"
domain-name = "mydomain"
tenant-name = "mytenantname"
username = "myuser"
password = "myPaSs"
region = "myreg"
ca-file = /etc/config/ca.crt
[Networking]
public-network-name = "myextnetname"
public-network-name = "myextnetname2"
internal-network-name = "myintnetname"
internal-network-name = "myintnetname2"
ipv6-support-disabled = true
[LoadBalancer]
create-monitor = "true"
monitor-delay = "2s"
monitor-timeout = "1s"
subnet-id = "my-subnet-id"
floating-network-id = "my-floating-network-id"
enable-ingress-hostname = true
[BlockStorage]
ignore-volume-microversion = false
rescan-on-resize = true`

		ccmConfig, err := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(ccmConfig)).To(Equal(ccmExpectedConfig))

		Expect(userAuthzUser.Exists()).To(BeTrue())
		Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

		Expect(scFast.Exists()).To(BeTrue())
		Expect(scFast.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		Expect(scSlow.Exists()).To(BeTrue())
	})
}

var _ = Describe("Module :: cloud-provider-openstack :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-openstack/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/candi/cloud-providers/openstack", "/deckhouse/ee/modules/030-cloud-provider-openstack/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-openstack/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/candi/cloud-providers/openstack", "/deckhouse/ee/modules/030-cloud-provider-openstack/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("Openstack with k8s 1.29", func() {
		openstackCheck(f, "1.29")
	})

	Context("Openstack", func() {
		openstackCheck(f, "1.26")
	})

	Context("Openstack with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack.internal.defaultStorageClass", `slowhdd`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			scFast := f.KubernetesGlobalResource("StorageClass", "fastssd")
			scSlow := f.KubernetesGlobalResource("StorageClass", "slowhdd")

			Expect(scFast.Exists()).To(BeTrue())
			Expect(scSlow.Exists()).To(BeTrue())

			Expect(scFast.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(scSlow.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Openstack bad config", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderOpenstack", badModuleValues)
			f.HelmRender()
		})

		It("Test should fail", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).ShouldNot(BeEmpty())
		})
	})

	Context("Unsupported Kubernetes version", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
			f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
			f.HelmRender()
		})

		It("CCM and CSI controller should not be present on unsupported Kubernetes versions", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "cloud-controller-manager").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "csi-controller").Exists()).To(BeFalse())
		})
	})

	Context("Openstack StorageClass topology disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack.storageClass.topologyEnabled", "false")
			f.HelmRender()
		})

		It("Everything must render properly and csi controller provisioner arg must have flag feature-gates=Topology=false", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cinderControllerPluginSS := f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "csi-controller")
			Expect(cinderControllerPluginSS.Exists()).To(BeTrue())
			Expect(cinderControllerPluginSS.Field("spec.template.spec.containers.0.args.3").String()).To(MatchYAML(`--feature-gates=Topology=false`))
		})
	})

	Context("Openstack ignoreVolumeMicroversion", func() {
		assertConfigSecretIgnoreMicroVer := func(f *Config, s string) {
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmConfig, err := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			sExp := fmt.Sprintf("ignore-volume-microversion = %s", s)
			Expect(ccmConfig).To(ContainSubstring(sExp))
		}

		Context("all kube versions", func() {
			Context("ignoreVolumeMicroversion disabled", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.27", "1.27"))
					f.ValuesSet("global.modulesImages", GetModulesImages())
					f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
					f.ValuesSetFromYaml("cloudProviderOpenstack.ignoreVolumeMicroversion", "false")
					f.HelmRender()
				})

				It("Should not render 'ignore-volume-microversion' in ccm config", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertConfigSecretIgnoreMicroVer(f, "false")
				})
			})

			Context("ignoreVolumeMicroversion enabled", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.27", "1.27"))
					f.ValuesSet("global.modulesImages", GetModulesImages())
					f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
					f.ValuesSetFromYaml("cloudProviderOpenstack.ignoreVolumeMicroversion", "true")
					f.HelmRender()
				})

				It("Should render 'ignore-volume-microversion = true' in ccm config", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					assertConfigSecretIgnoreMicroVer(f, "true")
				})
			})
		})
	})

	Context("Cloud data discoverer", func() {
		deployment := func(f *Config) object_store.KubeObject {
			return f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "cloud-data-discoverer")
		}

		assertEnv := func(f *Config, envName string) {
			d := deployment(f)
			Expect(d.Exists()).To(BeTrue())

			envs := d.Field("spec.template.spec.containers.0.env").Array()

			found := false
			for _, e := range envs {
				if e.Map()["name"].String() == envName {
					found = true
					break
				}
			}

			Expect(found).To(BeTrue())
		}

		assertSecretFieldExists := func(f *Config, field string) {
			s := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "cloud-data-discoverer")
			Expect(s.Exists()).To(BeTrue())

			Expect(s.Field(fmt.Sprintf("data.%s", field)).Exists()).To(BeTrue())
		}

		Context("with tenant name", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
				f.HelmRender()
			})

			It("Should render cloud data discoverer deployment with two containers", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := deployment(f)
				Expect(d.Exists()).To(BeTrue())

				Expect(d.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
				Expect(d.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			})

			It("Should render OS_TENANT_NAME env for first container", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertEnv(f, "OS_TENANT_NAME")
			})

			It("Should render 'tenantName' secret field", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertSecretFieldExists(f, "tenantName")
			})
		})

		Context("with tenant id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
				f.ValuesSetFromYaml("cloudProviderOpenstack.internal.connection", `
authURL: http://my.cloud.lalla/123/
username: myuser
password: myPaSs
domainName: mydomain
tenantID: mytenantid
caCert: mycacert
region: myreg
`)
				f.HelmRender()
			})

			It("Should render cloud data discoverer deployment with two containers", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := deployment(f)
				Expect(d.Exists()).To(BeTrue())

				Expect(d.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
				Expect(d.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			})

			It("Should render OS_TENANT_ID env for first container", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertEnv(f, "OS_TENANT_ID")
			})

			It("Should render 'tenantID' secret field", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertSecretFieldExists(f, "tenantID")
			})
		})

		Context("with ca cert", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
				f.ValuesSetFromYaml("cloudProviderOpenstack.internal.connection.caCert", `
multiline
ca
`)
				f.HelmRender()
			})

			It("Should render cloud data discoverer deployment with two containers", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := deployment(f)
				Expect(d.Exists()).To(BeTrue())

				Expect(d.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
				Expect(d.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			})

			It("Should render OS_CACERT env for first container", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertEnv(f, "OS_CACERT")
			})

			It("Should render 'ca.crt' secret field", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertSecretFieldExists(f, `ca\.crt`)
			})
		})

		Context("vertical-pod-autoscaler-crd module enabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler-crd"]`)
				f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
				f.HelmRender()
			})

			It("Should render VPA resource", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-openstack", "cloud-data-discoverer")
				Expect(d.Exists()).To(BeTrue())
			})
		})

		Context("vertical-pod-autoscaler-crd module disabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("global.enabledModules", `[]`)
				f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
				f.HelmRender()
			})

			It("Should render VPA resource", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-openstack", "cloud-data-discoverer")
				Expect(d.Exists()).To(BeFalse())
			})
		})
	})
})
