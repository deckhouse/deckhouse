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

type ArgV4 struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ControlPlaneComponentV4 struct {
	ExtraArgs []ArgV4 `yaml:"extraArgs,omitempty"`
}
type APIServerV4 struct {
	ControlPlaneComponentV4 `yaml:",inline"`
}

type ClusterConfigurationV4 struct {
	APIVersion          string      `yaml:"apiVersion"`
	Kind                string      `yaml:"kind"`
	EncryptionAlgorithm string      `yaml:"encryptionAlgorithm,omitempty"`
	APIServer           APIServerV4 `yaml:"apiServer"`
}

type ArgV3 struct {
	Name  string
	Value string
}

type ControlPlaneComponentV3 struct {
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

type APIServer struct {
	ControlPlaneComponentV3 `yaml:",inline"`
}

type ClusterConfigurationV3 struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	APIServer  APIServer `yaml:"apiServer"`
}

type PrefixedClaimOrExpression struct {
	Claim  string  `yaml:"claim"`
	Prefix *string `yaml:"prefix"`

	Expression string `yaml:"expression,omitempty"`
}

type ExtraClaimMapping struct {
	Key             string `yaml:"key"`
	ValueExpression string `yaml:"valueExpression"`
}

type AuthenticationConfigurationV1beta1 struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	JWT        []struct {
		Issuer struct {
			URL                  string   `yaml:"url"`
			DiscoveryURL         string   `yaml:"discoveryURL"`
			CertificateAuthority string   `yaml:"certificateAuthority"`
			Audiences            []string `yaml:"audiences"`
		} `yaml:"issuer"`
		ClaimMappings struct {
			Username PrefixedClaimOrExpression `yaml:"username"`
			Groups   PrefixedClaimOrExpression `yaml:"groups"`
			Extra    []ExtraClaimMapping       `yaml:"extra"`
		} `yaml:"claimMappings"`
	} `yaml:"jwt"`
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
    effectiveKubernetesVersion: "1.32"
    etcdServers:
      - https://192.168.199.186:2379
    mastersNode:
      - master-0
    pkiChecksum: checksum
    rolloutEpoch: 1857
`

	const defultAudience = "https://kubernetes.default.svc.cluster.local"

	const moduleValuesOnlyIssuer = `
internal:
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
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
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
`

	const apiServerWithOidcFull = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  audit: {}
apiserver:
  authn:
    oidcIssuerURL: https://dex.example.com
    oidcCA: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
`
	const apiServerWithOidcIssuerOnly = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  audit: {}
apiserver:
  authn:
    oidcIssuerURL: https://dex.example.com
`

	const apiServerWithOidcEmpty = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  audit: {}
apiserver:
  authn: {}
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://api.example.com",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://api.example.com,%s", defultAudience),
				}))
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://api.example.com",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://api.example.com,https://bob.com,%s", defultAudience),
				}))

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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://api.example.com",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://api.example.com,https://api.bob.com,%s", defultAudience),
				}))

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(7))
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(data, &config)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: defultAudience,
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://api.example.com,https://bob.com,https://flant.com,%s", defultAudience),
				}))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(7))
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://api.example.com",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://api.example.com,https://flant.ru,%s", defultAudience),
				}))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(7))
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://kubernetes.default.svc.cluster.local",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://flant.ru,%s", defultAudience),
				}))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(7))
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://kubernetes.default.svc.cluster.local",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: fmt.Sprintf("https://flant.com,%s", defultAudience),
				}))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).To(ContainSubstring("--service-account-issuer"))
				documents := strings.Split(string(kubeApiserver), "---")
				Expect(documents).To(HaveLen(7))
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
				var config ClusterConfigurationV4
				err = yaml.Unmarshal(kubeadmConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "service-account-issuer",
					Value: "https://kubernetes.default.svc.cluster.local",
				}))
				Expect(config.APIServer.ExtraArgs).To(ContainElement(ArgV4{
					Name:  "api-audiences",
					Value: "https://kubernetes.default.svc.cluster.local",
				}))

				// kube-apiserver.yaml.tpl - contains patches for kube-api pod, including patches for adding additional service-account-issuer
				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeApiserver).ToNot(ContainSubstring("--service-account-issuer"))
			})
		})
		Context("cluster is bootstrapped", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSet("global.clusterIsBootstrapped", true)
				f.HelmRender()
			})

			It("cronjob for etcd backup should be exist by default", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Cronjob", "kube-system", "d8-etcd-backup-039d00b17e10d07f52111429fc7d82e2c")
				Expect(s.Exists()).To(BeTrue())
			})
		})
		Context("apiserver oidc settings are set fully", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", apiServerWithOidcFull)
				f.HelmRender()
			})
			It("for issuer[0] should bet set discoveryURL, URL and certificateAuthority", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				authConfig, err := base64.StdEncoding.DecodeString(s.Field("data.extra-file-authentication-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config AuthenticationConfigurationV1beta1
				err = yaml.Unmarshal(authConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(config.APIVersion).To(Equal("apiserver.config.k8s.io/v1beta1"))
				Expect(config.JWT[0].Issuer.DiscoveryURL).To(Equal("https://dex.d8-user-authn.svc.cluster.local/.well-known/openid-configuration"))
				Expect(config.JWT[0].Issuer.URL).To(Equal("https://dex.example.com"))
				Expect(config.JWT[0].Issuer.CertificateAuthority).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n    \n"))
			})
			It("should include extra claim mappings for user-authn.deckhouse.io claims", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				authConfig, err := base64.StdEncoding.DecodeString(s.Field("data.extra-file-authentication-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config AuthenticationConfigurationV1beta1
				err = yaml.Unmarshal(authConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())

				// Verify extra claim mappings are present
				extraMappings := config.JWT[0].ClaimMappings.Extra
				Expect(extraMappings).To(HaveLen(3))

				// Check user-authn.deckhouse.io/name mapping
				Expect(extraMappings).To(ContainElement(ExtraClaimMapping{
					Key:             "user-authn.deckhouse.io/name",
					ValueExpression: "claims.name",
				}))

				// Check user-authn.deckhouse.io/preferred_username mapping
				Expect(extraMappings).To(ContainElement(ExtraClaimMapping{
					Key:             "user-authn.deckhouse.io/preferred_username",
					ValueExpression: "has(claims.preferred_username) ? claims.preferred_username : null",
				}))

				// Check user-authn.deckhouse.io/dex-provider mapping (from Dex federated_claims.connector_id)
				// Note: connector_id appears in id_token only when client requests federated:id scope
				// Without this scope, the mapping returns null and field won't appear in .user.extra
				// To enable: add --oidc-extra-scope=federated:id to OIDC client (e.g., kubelogin/oidc-login)
				Expect(extraMappings).To(ContainElement(ExtraClaimMapping{
					Key:             "user-authn.deckhouse.io/dex-provider",
					ValueExpression: "has(claims.federated_claims) && has(claims.federated_claims.connector_id) ? claims.federated_claims.connector_id : null",
				}))
			})
		})
		Context("apiserver oidc settings are set partially", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", apiServerWithOidcIssuerOnly)
				f.HelmRender()
			})
			It("for issuer[0] should bet set only discoveryURL and URL", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				authConfig, err := base64.StdEncoding.DecodeString(s.Field("data.extra-file-authentication-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config AuthenticationConfigurationV1beta1
				err = yaml.Unmarshal(authConfig, &config)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(config.APIVersion).To(Equal("apiserver.config.k8s.io/v1beta1"))
				Expect(config.JWT[0].Issuer.DiscoveryURL).To(Equal("https://dex.d8-user-authn.svc.cluster.local/.well-known/openid-configuration"))
				Expect(config.JWT[0].Issuer.URL).To(Equal("https://dex.example.com"))
				Expect(config.JWT[0].Issuer.CertificateAuthority).Should(BeEmpty())
			})
		})
		Context("apiserver oidc settings are empty", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", apiServerWithOidcEmpty)
				f.HelmRender()
			})
			It("extra-file-authentication-config.yaml should not be created", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				authConfig, err := base64.StdEncoding.DecodeString(s.Field("data.extra-file-authentication-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(authConfig).Should(BeEmpty())
			})
		})
	})

	Context("kubeadm config version selection", func() {
		testKubeadmVersion := func(k8sVersion, expectedApiVersion string) {
			testValues := fmt.Sprintf(`
internal:
  effectiveKubernetesVersion: "%s"
  etcdServers:
    - https://192.168.199.186:2379
  mastersNode:
    - master-0
  pkiChecksum: checksum
  rolloutEpoch: 1857
`, k8sVersion)

			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", testValues)
				f.HelmRender()
			})

			It(fmt.Sprintf("should use %s for Kubernetes %s", expectedApiVersion, k8sVersion), func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(secret.Exists()).To(BeTrue())

				kubeadmConfigData, err := base64.StdEncoding.DecodeString(secret.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(kubeadmConfigData)).ToNot(BeEmpty())

				if expectedApiVersion == "v1beta3" {
					var config ClusterConfigurationV3
					err = yaml.Unmarshal(kubeadmConfigData, &config)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(config.APIVersion).To(Equal("kubeadm.k8s.io/v1beta3"))
					Expect(config.Kind).To(Equal("ClusterConfiguration"))
				} else {
					var config ClusterConfigurationV4
					err = yaml.Unmarshal(kubeadmConfigData, &config)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(config.APIVersion).To(Equal("kubeadm.k8s.io/v1beta4"))
					Expect(config.Kind).To(Equal("ClusterConfiguration"))
				}
			})
		}

		Context("Kubernetes 1.30", func() {
			testKubeadmVersion("1.30", "v1beta3")
		})

		Context("Kubernetes 1.31", func() {
			testKubeadmVersion("1.31", "v1beta4")
		})

		Context("Kubernetes 1.32", func() {
			testKubeadmVersion("1.32", "v1beta4")
		})
	})

	Context("webhook configuration in v1beta4", func() {
		const webhookTestValues = `
internal:
  effectiveKubernetesVersion: "1.31"
  etcdServers:
    - https://192.168.199.186:2379
  mastersNode:
    - master-0
  pkiChecksum: checksum
  rolloutEpoch: 1857
  audit:
    webhookURL: "https://audit.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
apiserver:
  authz:
    webhookURL: "https://authz.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
  authn:
    webhookURL: "https://authn.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
    webhookCacheTTL: "5m"
`

		const v1beta3TestValues = `
internal:
  effectiveKubernetesVersion: "1.30"
  etcdServers:
    - https://192.168.199.186:2379
  mastersNode:
    - master-0
  pkiChecksum: checksum
  rolloutEpoch: 1857
  audit:
    webhookURL: "https://audit.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
apiserver:
  authz:
    webhookURL: "https://authz.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
  authn:
    webhookURL: "https://authn.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
    webhookCacheTTL: "5m"
`

		Context("v1beta4 with webhook parameters", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", webhookTestValues)
				f.HelmRender()
			})

			It("should include webhook parameters in v1beta4 configuration using array syntax", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(secret.Exists()).To(BeTrue())

				kubeadmConfigData, err := base64.StdEncoding.DecodeString(secret.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())

				configYaml := string(kubeadmConfigData)
				Expect(configYaml).To(ContainSubstring("apiVersion: kubeadm.k8s.io/v1beta4"))

				// v1beta4 uses array syntax with name/value pairs
				Expect(configYaml).To(ContainSubstring("- name: authorization-mode"))
				Expect(configYaml).To(ContainSubstring("value: Node,Webhook,RBAC"))
				Expect(configYaml).To(ContainSubstring("- name: authorization-webhook-config-file"))

				Expect(configYaml).To(ContainSubstring("- name: authentication-token-webhook-config-file"))
				Expect(configYaml).To(ContainSubstring("- name: authentication-token-webhook-cache-ttl"))
				Expect(configYaml).To(ContainSubstring("value: \"5m\""))

				Expect(configYaml).To(ContainSubstring("- name: audit-webhook-config-file"))

				// v1beta4 should NOT have the map syntax
				Expect(configYaml).ToNot(ContainSubstring("authorization-mode: Node,Webhook,RBAC"))
			})
		})

		Context("v1beta3 uses map syntax for webhook parameters", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", v1beta3TestValues)
				f.HelmRender()
			})

			It("should include webhook parameters using map syntax in v1beta3", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(secret.Exists()).To(BeTrue())

				kubeadmConfigData, err := base64.StdEncoding.DecodeString(secret.Field("data.kubeadm-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())

				configYaml := string(kubeadmConfigData)
				Expect(configYaml).To(ContainSubstring("apiVersion: kubeadm.k8s.io/v1beta3"))

				// v1beta3 uses map syntax (key: value) instead of array syntax
				Expect(configYaml).To(ContainSubstring("authorization-mode: Node,Webhook,RBAC"))
				Expect(configYaml).To(ContainSubstring("authorization-webhook-config-file: /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml"))
				Expect(configYaml).To(ContainSubstring("authentication-token-webhook-config-file: /etc/kubernetes/deckhouse/extra-files/authn-webhook-config.yaml"))
				Expect(configYaml).To(ContainSubstring("authentication-token-webhook-cache-ttl: \"5m\""))
				Expect(configYaml).To(ContainSubstring("audit-webhook-config-file: /etc/kubernetes/deckhouse/extra-files/audit-webhook-config.yaml"))

				// v1beta3 should NOT have the array syntax with name/value pairs
				Expect(configYaml).ToNot(ContainSubstring("- name: authorization-mode"))
				Expect(configYaml).ToNot(ContainSubstring("- name: authorization-webhook-config-file"))
			})
		})
	})
})
