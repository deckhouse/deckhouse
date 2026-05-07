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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

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
	Anonymous struct {
		Enabled    bool `yaml:"enabled"`
		Conditions []struct {
			Path string `yaml:"path"`
		} `yaml:"conditions"`
	} `yaml:"anonymous"`
}

func CountLinesContaining(content []byte, substring string) int {
	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, substring) {
			count++
		}
	}
	return count
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
  apiserver:
    publishAPI:
      ingress: {}
      loadBalancer: {}
  internal:
    effectiveKubernetesVersion: "1.32"
    etcdServers:
      - https://192.168.199.186:2379
    mastersNode:
      - master-0
    pkiChecksum: checksum
    rolloutEpoch: 1857
    nodesCount: 0
    kubeSchedulerExtenders: []
    authn: {}
    selfSignedCA: {}
`

	const defaultAudience = "https://kubernetes.default.svc.cluster.local"

	const moduleValuesOnlyIssuer = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  authn: {}
  selfSignedCA: {}
apiserver:
  serviceAccount:
    issuer: https://api.example.com
  publishAPI:
    ingress: {}
    loadBalancer: {}
`
	const moduleValuesIssuerAdditionalAudiences = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  authn: {}
  selfSignedCA: {}
apiserver:
  serviceAccount:
    issuer: https://api.example.com
    additionalAPIAudiences:
      - https://api.example.com
      - https://bob.com
  publishAPI:
    ingress: {}
    loadBalancer: {}
`

	const moduleValuesAdditionalIssuerOnly = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
`

	const apiServerWithOidcFull = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  pkiChecksum: checksum
  rolloutEpoch: 1857
  authn: {}
  selfSignedCA: {}
  audit: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
  audit: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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
  authn: {}
  selfSignedCA: {}
  audit: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
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

			It("should set issuer and default api-audiences", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--service-account-issuer=https://api.example.com",
					fmt.Sprintf("--api-audiences=https://api.example.com,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				serviceAccountEntries := 0
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries++
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(serviceAccountEntries).To(BeNumerically("<", 2))
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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--service-account-issuer=https://api.example.com",
					fmt.Sprintf("--api-audiences=https://api.example.com,https://bob.com,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				serviceAccountEntries := 0
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries++
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(serviceAccountEntries).To(BeNumerically("==", 1))

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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--service-account-issuer=https://api.example.com",
					"--service-account-issuer=https://api.bob.com",
					fmt.Sprintf("--api-audiences=https://api.example.com,https://api.bob.com,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				serviceAccountEntries := 0
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries++
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(serviceAccountEntries).To(BeNumerically("==", 2))

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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				defaultServiceAccountIssuer := fmt.Sprintf("--service-account-issuer=%s", defaultAudience)
				expectedCommands := []string{
					defaultServiceAccountIssuer,
					"--service-account-issuer=https://api.example.com",
					"--service-account-issuer=https://bob.com",
					fmt.Sprintf("--api-audiences=https://api.example.com,https://bob.com,https://flant.com,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				var serviceAccountEntries []string
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries = append(serviceAccountEntries, item)
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(len(serviceAccountEntries)).To(BeNumerically("==", 3))
				// service-account-issuer order matters, the first one must be the one we want to use to generate tokens
				Expect(serviceAccountEntries[0]).To(Equal(defaultServiceAccountIssuer))

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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				defaultServiceAccountIssuer := fmt.Sprintf("--service-account-issuer=%s", defaultAudience)
				expectedCommands := []string{
					"--service-account-issuer=https://api.example.com",
					defaultServiceAccountIssuer,
					"--service-account-issuer=https://flant.ru",
					fmt.Sprintf("--api-audiences=https://api.example.com,https://flant.ru,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				var serviceAccountEntries []string
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries = append(serviceAccountEntries, item)
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(len(serviceAccountEntries)).To(BeNumerically("==", 3))
				// service-account-issuer order matters, the first one must be the one we want to use to generate tokens
				Expect(serviceAccountEntries[0]).To(Equal("--service-account-issuer=https://api.example.com"))
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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
					"--service-account-issuer=https://flant.ru",
					fmt.Sprintf("--api-audiences=https://flant.ru,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				var serviceAccountEntries []string
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries = append(serviceAccountEntries, item)
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(len(serviceAccountEntries)).To(BeNumerically("==", 2))
				// service-account-issuer order matters, the first one must be the one we want to use to generate tokens
				Expect(serviceAccountEntries[0]).To(Equal("--service-account-issuer=https://kubernetes.default.svc.cluster.local"))
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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
					"--service-account-issuer=https://flant.com",
					fmt.Sprintf("--api-audiences=https://flant.com,%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				var serviceAccountEntries []string
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries = append(serviceAccountEntries, item)
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(len(serviceAccountEntries)).To(BeNumerically("==", 2))
				// service-account-issuer order matters, the first one must be the one we want to use to generate tokens
				Expect(serviceAccountEntries[0]).To(Equal("--service-account-issuer=https://kubernetes.default.svc.cluster.local"))
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

				kubeApiserver, err := base64.StdEncoding.DecodeString(s.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
					fmt.Sprintf("--api-audiences=%s", defaultAudience),
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				var serviceAccountEntries []string
				for _, item := range pod.Spec.Containers[0].Command {
					if strings.HasPrefix(item, "--service-account-issuer=") {
						serviceAccountEntries = append(serviceAccountEntries, item)
					}
				}
				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				Expect(len(serviceAccountEntries)).To(BeNumerically("==", 1))
				// service-account-issuer order matters, the first one must be the one we want to use to generate tokens
				Expect(serviceAccountEntries[0]).To(Equal("--service-account-issuer=https://kubernetes.default.svc.cluster.local"))
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
				Expect(config.JWT[0].Issuer.CertificateAuthority).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"))
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
			It("extra-file-authentication-config.yaml must exist and contain anonymous settings", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				s := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(s.Exists()).To(BeTrue())
				authConfig, err := base64.StdEncoding.DecodeString(s.Field("data.extra-file-authentication-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				var config AuthenticationConfigurationV1beta1
				err = yaml.Unmarshal(authConfig, &config)
				Expect(config.APIVersion).To(Equal("apiserver.config.k8s.io/v1beta1"))
				Expect(config.JWT).Should(BeEmpty())
				Expect(config.Anonymous.Enabled).To(Equal(true))
				Expect(config.Anonymous.Conditions).To(ContainElements(
					HaveField("Path", "/livez"),
					HaveField("Path", "/healthz"),
				))
			})
		})
	})

	Context("webhook configuration in apiserver", func() {
		const webhookTestValues = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  mastersNode:
    - master-0
  pkiChecksum: checksum
  rolloutEpoch: 1857
  authn: {}
  selfSignedCA: {}
  audit:
    webhookURL: "https://audit.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
  authz:
    webhookURL: "https://authz.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
  authn:
    webhookURL: "https://authn.example.com"
    webhookCA: "LS0tLS1CRUdJTi..."
    webhookCacheTTL: "5m"
`

		const webhookAuthzMissingCATestValues = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  mastersNode:
    - master-0
  pkiChecksum: checksum
  rolloutEpoch: 1857
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
  authz:
    webhookURL: "https://authz.example.com"
`

		Context("apiserver with webhook parameters", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", webhookTestValues)
				f.HelmRender()
			})

			It("should include webhook parameters apiserver manifest and authorization config", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				secret := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
				Expect(secret.Exists()).To(BeTrue())

				// structured authorization config file should be present in extra-files secret
				authzConfigData, err := base64.StdEncoding.DecodeString(secret.Field("data.extra-file-authorization-config\\.yaml").String())
				Expect(err).ShouldNot(HaveOccurred())
				authzConfigYaml := string(authzConfigData)
				Expect(authzConfigYaml).To(ContainSubstring("kind: AuthorizationConfiguration"))
				// Ensure authz webhook is fail-closed but bypasses core control-plane identities to avoid deadlocks.
				Expect(authzConfigYaml).To(ContainSubstring("failurePolicy: Deny"))
				Expect(authzConfigYaml).To(ContainSubstring("matchConditions:"))
				Expect(authzConfigYaml).To(ContainSubstring(`expression: '!(request.user in ["system:aggregator", "system:kube-aggregator", "system:kube-controller-manager", "system:kube-scheduler", "kubernetes-admin", "kube-apiserver-kubelet-client", "capi-controller-manager", "system:volume-scheduler"])'`))
				Expect(authzConfigYaml).To(ContainSubstring(`expression: '!(request.user.startsWith("system:node:"))'`))
				Expect(authzConfigYaml).To(ContainSubstring(`expression: '!(request.user.startsWith("system:serviceaccount:kube-system:"))'`))
				Expect(authzConfigYaml).To(ContainSubstring(`expression: '!(request.user.startsWith("system:serviceaccount:d8-"))'`))

				kubeApiserver, err := base64.StdEncoding.DecodeString(secret.Field("data.kube-apiserver\\.yaml\\.tpl").String())
				Expect(err).ShouldNot(HaveOccurred())
				var pod corev1.Pod
				expectedCommands := []string{
					"--authorization-config=/etc/kubernetes/deckhouse/extra-files/authorization-config.yaml",
					"--authentication-token-webhook-config-file=/etc/kubernetes/deckhouse/extra-files/authn-webhook-config.yaml",
					"--authentication-token-webhook-cache-ttl=5m",
					"--audit-webhook-config-file=/etc/kubernetes/deckhouse/extra-files/audit-webhook-config.yaml",
				}
				err = yaml.Unmarshal(kubeApiserver, &pod)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))
				for _, command := range pod.Spec.Containers[0].Command {
					Expect(command).ToNot(ContainSubstring("authorization-mode"))
					Expect(command).ToNot(ContainSubstring("authorization-webhook-config-file"))
				}
				Expect(pod.Spec.SecurityContext).ToNot(BeNil())
				Expect(pod.Spec.SecurityContext.SeccompProfile).ToNot(BeNil())
				Expect(pod.Spec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
				Expect(pod.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(BeEquivalentTo(3))
				Expect(pod.Spec.Containers[0].ReadinessProbe.PeriodSeconds).To(BeEquivalentTo(1))
				Expect(pod.Spec.Containers[0].ReadinessProbe.TimeoutSeconds).To(BeEquivalentTo(15))
				Expect(pod.Spec.Containers[0].LivenessProbe.FailureThreshold).To(BeEquivalentTo(8))
				Expect(pod.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).To(BeEquivalentTo(10))
				Expect(pod.Spec.Containers[0].LivenessProbe.TimeoutSeconds).To(BeEquivalentTo(15))
				Expect(pod.Spec.Containers[0].StartupProbe.FailureThreshold).To(BeEquivalentTo(24))
				Expect(pod.Spec.Containers[0].StartupProbe.InitialDelaySeconds).To(BeEquivalentTo(10))
				Expect(pod.Spec.Containers[0].StartupProbe.TimeoutSeconds).To(BeEquivalentTo(15))

			})
		})

		Context("apiserver with authz webhookURL but without webhookCA", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("controlPlaneManager", webhookAuthzMissingCATestValues)
				f.HelmRender()
			})

			It("should fail helm render with explicit error", func() {
				Expect(f.RenderError).Should(HaveOccurred())
				Expect(f.RenderError.Error()).To(ContainSubstring("controlPlaneManager.apiserver.authz.webhookCA is required"))
			})
		})

		Context("terminated-pod-gc-threshold based on node count", func() {
			testTerminatedPodGcThreshold := func(nodesCount int, expectedThreshold string) {
				Context(fmt.Sprintf("with %d nodes", nodesCount), func() {
					const testValuesTemplate = `
internal:
  effectiveKubernetesVersion: "1.32"
  etcdServers:
    - https://192.168.199.186:2379
  mastersNode:
    - master-0
  pkiChecksum: checksum
  rolloutEpoch: 1857
  nodesCount: %d
  kubeSchedulerExtenders: []
  authn: {}
  selfSignedCA: {}
apiserver:
  publishAPI:
    ingress: {}
    loadBalancer: {}
`

					testValues := fmt.Sprintf(testValuesTemplate, nodesCount)

					BeforeEach(func() {
						f.ValuesSetFromYaml("controlPlaneManager", testValues)
						f.HelmRender()
					})

					It(fmt.Sprintf("should set terminated-pod-gc-threshold to %s", expectedThreshold), func() {
						Expect(f.RenderError).ShouldNot(HaveOccurred())

						secret := f.KubernetesResource("Secret", "kube-system", "d8-control-plane-manager-config")
						Expect(secret.Exists()).To(BeTrue())
						kubeApiserver, err := base64.StdEncoding.DecodeString(secret.Field("data.kube-controller-manager\\.yaml\\.tpl").String())
						Expect(err).ShouldNot(HaveOccurred())
						var pod corev1.Pod
						expectedCommands := []string{
							fmt.Sprintf("--terminated-pod-gc-threshold=%s", expectedThreshold),
						}
						err = yaml.Unmarshal(kubeApiserver, &pod)
						Expect(err).ShouldNot(HaveOccurred())

						Expect(pod.Spec.Containers[0].Command).To(ContainElements(expectedCommands))

					})
				})
			}

			// Test cases for different node counts with Kubernetes-1.32
			testTerminatedPodGcThreshold(0, "1000") // default value
			testTerminatedPodGcThreshold(50, "1000")
			testTerminatedPodGcThreshold(99, "1000")
			testTerminatedPodGcThreshold(100, "3000")
			testTerminatedPodGcThreshold(150, "3000")
			testTerminatedPodGcThreshold(299, "3000")
			testTerminatedPodGcThreshold(300, "6000")
			testTerminatedPodGcThreshold(500, "6000")
		})
	})

	Context("rootKubeconfigSymlink (control-plane-manager module values)", func() {
		Context("when user-authz is enabled and controlPlaneManager.rootKubeconfigSymlink is false", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["user-authz"]`)
				f.ValuesSet("controlPlaneManager.rootKubeconfigSymlink", false)
				f.HelmRender()
			})

			It("should set NODE_ADMIN_KUBECONFIG env var to false in DaemonSet", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				ds := f.KubernetesResource("DaemonSet", "kube-system", "d8-control-plane-manager")
				Expect(ds.Exists()).To(BeTrue())

				containers := ds.Field("spec.template.spec.containers").Array()
				foundEnv := false
				for _, container := range containers {
					if container.Get("name").String() != "control-plane-manager" {
						continue
					}
					for _, env := range container.Get("env").Array() {
						if env.Get("name").String() == "NODE_ADMIN_KUBECONFIG" {
							foundEnv = true
							Expect(env.Get("value").String()).To(Equal("false"))
						}
					}
				}
				Expect(foundEnv).To(BeTrue())
			})
		})

		Context("when user-authz is enabled and controlPlaneManager.rootKubeconfigSymlink is true", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["user-authz"]`)
				f.ValuesSet("controlPlaneManager.rootKubeconfigSymlink", true)
				f.HelmRender()
			})

			It("should not set NODE_ADMIN_KUBECONFIG env var in DaemonSet", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				ds := f.KubernetesResource("DaemonSet", "kube-system", "d8-control-plane-manager")
				Expect(ds.Exists()).To(BeTrue())

				containers := ds.Field("spec.template.spec.containers").Array()
				for _, container := range containers {
					if container.Get("name").String() != "control-plane-manager" {
						continue
					}
					for _, env := range container.Get("env").Array() {
						Expect(env.Get("name").String()).ToNot(Equal("NODE_ADMIN_KUBECONFIG"))
					}
				}
			})
		})

		Context("when user-authz is not enabled but controlPlaneManager.rootKubeconfigSymlink is false", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.rootKubeconfigSymlink", false)
				f.HelmRender()
			})

			It("should not set NODE_ADMIN_KUBECONFIG (parameter ignored without user-authz module)", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				ds := f.KubernetesResource("DaemonSet", "kube-system", "d8-control-plane-manager")
				Expect(ds.Exists()).To(BeTrue())

				containers := ds.Field("spec.template.spec.containers").Array()
				for _, container := range containers {
					if container.Get("name").String() != "control-plane-manager" {
						continue
					}
					for _, env := range container.Get("env").Array() {
						Expect(env.Get("name").String()).ToNot(Equal("NODE_ADMIN_KUBECONFIG"))
					}
				}
			})
		})
	})

	// The decision (target ClusterRole + whether to render supplement) is made by the
	// reconcile_kubeadm_cluster_admins_binding hook and published into Helm values. The template
	// reads .Values.controlPlaneManager.internal.{kubeadmClusterAdminsTargetRoleName,kubeadmClusterAdminsSupplementEnabled}
	// verbatim, so these tests drive the template directly via those internal values.
	Context("kubeadm ClusterRoleBinding for admin.conf", func() {
		Context("hook decision: target=cluster-admin, supplement=false (user-authz off)", func() {
			BeforeEach(func() {
				// schema defaults: target=cluster-admin, supplementEnabled=false; render with no overrides.
				f.HelmRender()
			})

			It("should bind kubeadm:cluster-admins to cluster-admin and not render the supplement", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				crb := f.KubernetesResource("ClusterRoleBinding", "", "kubeadm:cluster-admins")
				Expect(crb.Exists()).To(BeTrue())
				Expect(crb.Field("roleRef.name").String()).To(Equal("cluster-admin"))

				sup := f.KubernetesResource("ClusterRoleBinding", "", "d8:control-plane-manager:kubeadm-cluster-admins-supplement")
				Expect(sup.Exists()).To(BeFalse())
				supCR := f.KubernetesResource("ClusterRole", "", "d8:control-plane-manager:admin-kubeconfig-supplement")
				Expect(supCR.Exists()).To(BeFalse())
			})
		})

		Context("hook decision: target=cluster-admin, supplement=true (user-authz on, but at least one of bootstrap/CR-presence gates is false)", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName", "cluster-admin")
				f.ValuesSet("controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled", true)
				f.HelmRender()
			})

			It("should keep cluster-admin on main binding but render the supplement (purely additive on the same group)", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				main := f.KubernetesResource("ClusterRoleBinding", "", "kubeadm:cluster-admins")
				Expect(main.Exists()).To(BeTrue())
				Expect(main.Field("roleRef.name").String()).To(Equal("cluster-admin"))

				sup := f.KubernetesResource("ClusterRoleBinding", "", "d8:control-plane-manager:kubeadm-cluster-admins-supplement")
				Expect(sup.Exists()).To(BeTrue())
				Expect(sup.Field("roleRef.name").String()).To(Equal("d8:control-plane-manager:admin-kubeconfig-supplement"))

				supCR := f.KubernetesResource("ClusterRole", "", "d8:control-plane-manager:admin-kubeconfig-supplement")
				Expect(supCR.Exists()).To(BeTrue())
			})
		})

		Context("hook decision: target=user-authz:cluster-admin, supplement=true (all three gates satisfied)", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName", "user-authz:cluster-admin")
				f.ValuesSet("controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled", true)
				f.HelmRender()
			})

			It("should bind kubeadm:cluster-admins to user-authz:cluster-admin and add the supplement binding", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				main := f.KubernetesResource("ClusterRoleBinding", "", "kubeadm:cluster-admins")
				Expect(main.Exists()).To(BeTrue())
				Expect(main.Field("roleRef.name").String()).To(Equal("user-authz:cluster-admin"))

				sup := f.KubernetesResource("ClusterRoleBinding", "", "d8:control-plane-manager:kubeadm-cluster-admins-supplement")
				Expect(sup.Exists()).To(BeTrue())
				Expect(sup.Field("roleRef.name").String()).To(Equal("d8:control-plane-manager:admin-kubeconfig-supplement"))

				supCR := f.KubernetesResource("ClusterRole", "", "d8:control-plane-manager:admin-kubeconfig-supplement")
				Expect(supCR.Exists()).To(BeTrue())
				Expect(supCR.Field("rules").String()).To(ContainSubstring("control-plane.deckhouse.io"))
				Expect(supCR.Field("rules").String()).To(ContainSubstring("controlplanenodes"))
				Expect(supCR.Field("rules").String()).To(ContainSubstring("controlplaneoperations"))
			})
		})
	})
})
