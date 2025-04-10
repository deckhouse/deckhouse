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
	"fmt"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

type Arg struct {
	Name  string
	Value string
}

type ControlPlaneComponent struct {
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

type APIServer struct {
	ControlPlaneComponent `yaml:",inline"`
}

type ClusterConfiguration struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	APIServer  APIServer `yaml:"apiServer"`
}

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

	const defultAudience = "https://kubernetes.default.svc.cluster.local"

	const moduleValuesOnlyIssuer = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    issuer: https://api.example.com
`
	const moduleValuesIssuerAdditionalAudiences = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    issuer: https://api.example.com
    additionalAPIAudiences:
      - https://api.example.com
      - https://bob.com
`

	const moduleValuesAdditionalIssuerOnly = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    issuer: https://api.example.com
    additionalAPIIssuers:
      - https://api.bob.com
`

	const moduleValuesCombo = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    additionalAPIIssuers:
      - https://api.example.com
      - https://bob.com
    additionalAPIAudiences:
      - https://flant.com
`

	const moduleValuesSuperCombo = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    issuer: https://api.example.com
    additionalAPIIssuers:
      - https://kubernetes.default.svc.cluster.local
      - https://flant.ru
    additionalAPIAudiences:
      - https://kubernetes.default.svc.cluster.local
      - https://flant.ru
`

	const additionalAPIIssuersSuperComboWithDublicates = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    issuer: https://kubernetes.default.svc.cluster.local
    additionalAPIIssuers:
      - https://kubernetes.default.svc.cluster.local
      - https://flant.ru
    additionalAPIAudiences:
      - https://kubernetes.default.svc.cluster.local
      - https://flant.ru
`
	const additionalAPIIssuersSuperComboWithDublicates2 = `
internal:
  effectiveKubernetesVersion: "1.29"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
apiserver:
  serviceAccount:
    additionalAPIIssuers:
      - https://kubernetes.default.svc.cluster.local
      - https://flant.com
    additionalAPIAudiences:
      - https://kubernetes.default.svc.cluster.local
      - https://flant.com
`

	const emptyApiserverConfig = `
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

	Context("Image Holders", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("image holders must be properly named", func() {
			ds := f.KubernetesResource("daemonset", "kube-system", "d8-control-plane-manager")
			Expect(ds.Exists()).To(BeTrue())
			containers := ds.Field("spec.template.spec.containers").Array()
			var containerNames []string
			for _, c := range containers {
				containerNames = append(containerNames, c.Get("name").String())
			}
			Expect(slices.Contains(containerNames, "image-holder-kube-apiserver")).To(Equal(true))
		})
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
				// fake *-crd modules are required for backward compatibility with lib_helm library
				// TODO: remove fake crd modules
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
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
	Context("apiserver tests", func() {
		Context("only apiserver.serviceAccount.issuer", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", moduleValuesOnlyIssuer)
				f.HelmRender()
			})

			It("should set issuer and default api-audiencesr", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				data, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://api.example.com"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://api.example.com,%s", defultAudience)))
			})
		})

		Context("apiserver.serviceAccount.issuer with apiserver.serviceAccount.additionalAPIAudiences", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", moduleValuesIssuerAdditionalAudiences)
				f.HelmRender()
			})

			It("should set issuer and additionalAPIAudiences", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				data, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://api.example.com"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://api.example.com,https://bob.com,%s", defultAudience)))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).ToNot(ContainSubstring("--service-account-issuer"))
			})
		})

		Context("apiserver.serviceAccount.issuer with apiserver.serviceAccount.additionalAPIIssuers: A", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", moduleValuesAdditionalIssuerOnly)
				f.HelmRender()
			})

			It("should set issuer with additionalAPIIssuers in kube-apiserver.yaml.tpl", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				data, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://api.example.com"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://api.example.com,https://api.bob.com,%s", defultAudience)))

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(8))
				podWithExtraArgs := []byte(documents[6])
				var pod corev1.Pod
				expectedServiceAccountIssuers := []string{
					"--service-account-issuer=https://api.bob.com",
				}
				err = yaml.Unmarshal(podWithExtraArgs, &pod)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pod.Spec.Containers[0].Args).To(Equal(expectedServiceAccountIssuers))
			})
		})

		Context("apiserver.serviceAccount.issuer with apiserver.serviceAccount.additionalAPIIssuers: B", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", moduleValuesCombo)
				f.HelmRender()
			})

			It("should set issuer with additionalAPIIssuers in kube-apiserver.yaml.tpl", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				data, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", defultAudience))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://api.example.com,https://bob.com,https://flant.com,%s", defultAudience)))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(8))
				podWithExtraArgs := []byte(documents[6])
				var pod corev1.Pod
				expectedServiceAccountIssuers := []string{
					"--service-account-issuer=https://api.example.com",
					"--service-account-issuer=https://bob.com",
				}
				err = yaml.Unmarshal(podWithExtraArgs, &pod)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pod.Spec.Containers[0].Args).To(Equal(expectedServiceAccountIssuers))
			})
		})

		Context("apiserver.serviceAccount.issuer with additionalAPIIssuers and additionalAPIAudiences (super combo)", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", moduleValuesSuperCombo)
				f.HelmRender()
			})

			It("should set issuer, additional issuers and audiences", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				// kubeadm-config.yaml
				kubeadmConfig, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://api.example.com"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://api.example.com,https://flant.ru,%s", defultAudience)))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(8))
				podWithExtraArgs := []byte(documents[6])
				var pod corev1.Pod
				expectedServiceAccountIssuers := []string{
					"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
					"--service-account-issuer=https://flant.ru",
				}
				err = yaml.Unmarshal(podWithExtraArgs, &pod)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pod.Spec.Containers[0].Args).To(Equal(expectedServiceAccountIssuers))
			})
		})

		Context("duplicate handling scenario: A", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", additionalAPIIssuersSuperComboWithDublicates)
				f.HelmRender()
			})

			It("should set issuer, additional issuers and audiences without duplicates", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				// kubeadm-config.yaml
				kubeadmConfig, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://kubernetes.default.svc.cluster.local"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://flant.ru,%s", defultAudience)))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(8))
				podWithExtraArgs := []byte(documents[6])
				var pod corev1.Pod
				expectedServiceAccountIssuers := []string{
					"--service-account-issuer=https://flant.ru",
				}
				err = yaml.Unmarshal(podWithExtraArgs, &pod)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pod.Spec.Containers[0].Args).To(Equal(expectedServiceAccountIssuers))
			})
		})
		Context("duplicate handling scenario: B", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", additionalAPIIssuersSuperComboWithDublicates2)
				f.HelmRender()
			})

			It("should set issuer, additional issuers and audiences without duplicates", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				// kubeadm-config.yaml
				kubeadmConfig, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://kubernetes.default.svc.cluster.local"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", fmt.Sprintf("https://flant.com,%s", defultAudience)))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(8))
				podWithExtraArgs := []byte(documents[6])
				var pod corev1.Pod
				expectedServiceAccountIssuers := []string{
					"--service-account-issuer=https://flant.com",
				}
				err = yaml.Unmarshal(podWithExtraArgs, &pod)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pod.Spec.Containers[0].Args).To(Equal(expectedServiceAccountIssuers))
			})
		})
		Context("empty apiserver configuration", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", emptyApiserverConfig)
				f.HelmRender()
			})

			It("should set default issuer and audience", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				// kubeadm-config.yaml
				kubeadmConfig, err := base64.StdEncoding.DecodeString(s.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config ClusterConfiguration
				err = yaml.Unmarshal(kubeadmConfig, &config)
				// fmt.Println(string(kubeadmConfig))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("service-account-issuer", "https://kubernetes.default.svc.cluster.local"))
				Expect(config.APIServer.ExtraArgs).To(HaveKeyWithValue("api-audiences", "https://kubernetes.default.svc.cluster.local"))
				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).ToNot(ContainSubstring("--service-account-issuer"))
			})
		})
	})
})
