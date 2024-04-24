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

package hooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("helm :: hooks :: deprecated_versions ::", func() {
	f := HookExecutionConfigInit(`{"global" : {"discovery": {"kubernetesVersion": "1.22.3"}}}`, "")
	Context("helm3 release with deprecated versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(helm3ReleaseWithDeprecated), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("appns").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
		})

		Context("check for k8s version 1.21", func() {
			BeforeEach(func() {
				f.ValuesSet("global.discovery.kubernetesVersion", "1.21.8")
				f.RunGoHook()
			})
			It("must have metric with deprecated resource", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(5))
				for _, metric := range metrics {
					switch metric.Name {
					case "helm_releases_count":
						if metric.Labels["helm_version"] == "3" {
							Expect(*metric.Value).To(Equal(float64(1)))
						}

					case "resource_versions_compatibility":
						Expect(*metric.Value).To(Equal(float64(1))) // 1 means deprecated
						Expect(metric.Labels["k8s_version"]).To(Equal("1.22"))
						Expect(metric.Labels["helm_release_namespace"]).To(Equal("appns")) // we check namespace injection, because the release contains 'default' namespace

						switch metric.Labels["api_version"] {
						case "networking.k8s.io/v1beta1":
							Expect(metric.Labels["kind"]).To(Equal("Ingress"))
						case "apiextensions.k8s.io/v1beta1":
							Expect(metric.Labels["kind"]).To(Equal("CustomResourceDefinition"))
						default:
							Fail("unknown api version")
						}
					}
				}
			})
		})

		// check delta for the current version + 2
		Context("check for k8s version 1.20", func() {
			BeforeEach(func() {
				f.ValuesSet("global.discovery.kubernetesVersion", "1.20.4")
				f.RunGoHook()
			})
			It("must have metric with deprecated resource for 1.22 version", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(5))
				for _, metric := range metrics {
					switch metric.Name {
					case "helm_releases_count":
						if metric.Labels["helm_version"] == "3" {
							Expect(*metric.Value).To(Equal(float64(1)))
						}

					case "resource_versions_compatibility":
						Expect(*metric.Value).To(Equal(float64(1))) // 1 means deprecated
						Expect(metric.Labels["k8s_version"]).To(Equal("1.22"))
						Expect(metric.Labels["helm_release_namespace"]).To(Equal("appns")) // we check namespace injection, because the release contains 'default' namespace

						switch metric.Labels["api_version"] {
						case "networking.k8s.io/v1beta1":
							// deprecated in the 1.22: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#ingress-v122
							Expect(metric.Labels["kind"]).To(Equal("Ingress"))
						}
					}
				}
			})
		})

		Context("check for k8s version 1.22", func() {
			BeforeEach(func() {
				f.ValuesSet("global.discovery.kubernetesVersion", "1.22.5")
				f.RunGoHook()
			})
			It("must have metric with deprecated resource", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(5))
				for _, metric := range metrics {
					switch metric.Name {
					case "helm_releases_count":
						if metric.Labels["helm_version"] == "3" {
							Expect(*metric.Value).To(Equal(float64(1)))
						}

					case "resource_versions_compatibility":
						Expect(*metric.Value).To(Equal(float64(2))) // 2 means unsupported
						Expect(metric.Labels["k8s_version"]).To(Equal("1.22"))

						switch metric.Labels["api_version"] {
						case "networking.k8s.io/v1beta1":
							Expect(metric.Labels["kind"]).To(Equal("Ingress"))
						case "apiextensions.k8s.io/v1beta1":
							Expect(metric.Labels["kind"]).To(Equal("CustomResourceDefinition"))
						default:
							Fail("unknown api version")
						}
					}
				}
			})
		})
	})

	Context("helm3 release without deprecated apis", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(helm3ReleaseWithoutDeprecated), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("default").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()
		})
		It("must have no metrics about deprecation", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(3)) // expire, count v3 and count v2
			for _, metric := range metrics {
				switch metric.Name {
				case "resource_versions_compatibility":
					Fail("must have no compatibility metrics")
				}
			}
		})
	})

	Context("helm2 release with deprecated versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var cm corev1.ConfigMap
			_ = yaml.Unmarshal([]byte(helm2ReleaseWithDeprecated), &cm)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("default").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			// also add helm2 release with 'proto: cannot parse invalid wire-format data' error here, we have to skip it
			_ = yaml.Unmarshal([]byte(helm2ReleaseWithInvalidWireFormat), &cm)

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("kube-system").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()

		})
		It("must have metric with deprecated resource", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(4))
			for _, metric := range metrics {
				switch metric.Name {
				case "helm_releases_count":
					if metric.Labels["helm_version"] == "2" {
						Expect(*metric.Value).To(Equal(float64(1)))
					}

				case "resource_versions_compatibility":
					Expect(metric.Labels["api_version"]).To(Equal("networking.k8s.io/v1beta1"))
					Expect(metric.Labels["kind"]).To(Equal("Ingress"))
					Expect(*metric.Value).To(Equal(float64(2)))
				}
			}
		})
	})

	Context("Should not check not deployed releases", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(helm3NotDeployed), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("default").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()

		})
		It("must have no metric with deprecated resource", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(3))
			for _, metric := range metrics {
				if metric.Name == "resource_versions_compatibility" {
					Fail("shouldn't find deprecated metrics")
				}
			}
		})
	})

	Context("Release with doubled fields", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(releaseWithDoubleFields), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("default").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()

		})
		It("Must be valid and have no deprecated resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			out, _ := io.ReadAll(f.LogrusOutput)
			Expect(string(out)).ToNot(ContainSubstring("manifest read error"))
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(3))
			for _, metric := range metrics {
				if metric.Name == "resource_versions_compatibility" {
					Fail("shouldn't find deprecated metrics")
				}
			}
		})
	})
})

var _ = Describe("helm :: hooks :: automatic kubernetes version ::", func() {
	var (
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "Automatic"
`
		stateAutomatic = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration))

		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "1.25"
`
		stateConcreteVersion = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration))
	)

	f := HookExecutionConfigInit("{\"global\": {\"discovery\": {\"kubernetesVersion\": \"1.21.3\"}}}", "{}")
	Context("helm3 release with deprecated versions", func() {
		Context("check for kubernetesVersion: \"Automatic\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))

				var sec corev1.Secret
				_ = yaml.Unmarshal([]byte(helm3ReleaseWithDeprecated), &sec)

				_, err := dependency.TestDC.MustGetK8sClient().
					CoreV1().
					Secrets("appns").
					Create(context.TODO(), &sec, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				f.RunGoHook()
			})

			It("must have autoK8sVersion", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(K8sVersionsWithDeprecations); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}

				Expect(k8sVersion).To(Equal("1.22"))
			})
		})

		Context("check for kubernetesVersion: \"1.25\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateConcreteVersion))
				var sec corev1.Secret
				_ = yaml.Unmarshal([]byte(helm3ReleaseWithDeprecated), &sec)

				_, err := dependency.TestDC.MustGetK8sClient().
					CoreV1().
					Secrets("appns").
					Create(context.TODO(), &sec, metav1.CreateOptions{})
				Expect(err).To(BeNil())
				f.RunGoHook()
			})

			It("autoK8sVersion must be empty", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(K8sVersionsWithDeprecations); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				Expect(k8sVersion).To(BeEmpty())
			})
		})

		Context("check for empty \"ClusterConfiguration\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunGoHook()
			})

			It("must execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})
		})
	})

	Context("helm3 release without deprecated apis", func() {
		Context("check for kubernetesVersion: \"Automatic\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))

				var sec corev1.Secret
				_ = yaml.Unmarshal([]byte(helm3ReleaseWithoutDeprecated), &sec)

				_, err := dependency.TestDC.MustGetK8sClient().
					CoreV1().
					Secrets("default").
					Create(context.TODO(), &sec, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				f.RunGoHook()
			})

			It("autoK8sVersion must be empty", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(K8sVersionsWithDeprecations); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				Expect(k8sVersion).To(BeEmpty())
			})
		})
	})

	Context("helm2 release with deprecated versions", func() {
		Context("check for kubernetesVersion: \"Automatic\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))

				var cm corev1.ConfigMap
				_ = yaml.Unmarshal([]byte(helm2ReleaseWithDeprecated), &cm)

				_, err := dependency.TestDC.MustGetK8sClient().
					CoreV1().
					ConfigMaps("default").
					Create(context.TODO(), &cm, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				f.RunGoHook()

			})

			It("must have autoK8sVersion", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(K8sVersionsWithDeprecations); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}

				Expect(k8sVersion).To(Equal("1.22"))
			})
		})
	})

	Context("release with doubled fields", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))

			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(releaseWithDoubleFields), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("default").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunGoHook()

		})

		It("must be valid and have no deprecated resources", func() {
			Expect(f).To(ExecuteSuccessfully())

			var k8sVersion string
			if val, exists := requirements.GetValue(K8sVersionsWithDeprecations); exists {
				k8sVersion = fmt.Sprintf("%v", val)
			}
			Expect(k8sVersion).To(BeEmpty())
		})
	})
})

const helm3ReleaseWithDeprecated = `
apiVersion: v1
data:
  release: SDRzSUFBQUFBQUFDLzVSWVczUGl1TGIrS3lyUHl6bTFTUnBNNkFsVXpRT213WmlBazBEaTI4bXVVN0lzYkFYWjhsZ3lZTHI2disrU2JBTko5OHl1ZVVqRmx0Zjk4cTBsdm1zWlRMRTIwcmFNTWFaMU5KSnRtVGI2cm0xSndjWC9Semluck1LUk50TDBycTdmZE85dTlNRkw3MjdVKzMzVTdkN2VEL3VEMzM4ZjN2Y0RyYU5SK0E4WklreXhVS1RxaGFPQzVJS3dUQnRwVnNZRnBCUWdsdWFTU090b1hFQlJjbTJrblZYODZHZ29nWVdRNXFaWXdBZ0tLSjh2SG1rZGJZOExYc3ZzM3ZadXV6OXBHb001cGlsUWdzQ1dGZUNoREhHUllZRzUxdEZnVHB5emdMMnVUdkxMU2UrMjkxWEpGRlV1VmNJOHB3UkJKZnBIUjZNTTdiUlJWbExhMFFST2N3cWwxTkgvblUwOEgzNGhXVnhnem04cm1GSnBvM0pGOHowamQxS25RanJkaCs4c0RsT25HK25EQ3VxMERLcEI4VGhmbGRDOTMwZnZNeEo0NjJUMW5CZlFIZXdlaWJFSTA2Z0tQUHMwU1hzME1tYzczMXNuanpHTExYT1FoTzdyVjh0MDdueTNkd2pOVndIZFFZWlM1NFFPOHZ1c0ROTmgxL2ZXZWFnUFRvcG5NbzVoM3lHQnR5Z0RiMDFSTmNqRGFwaTNPcGFwelgzUFBqMlNNWUg2c0F3cTZ3SDFEZXFmMklNMU1hcklQVktrbnNmQ01wTTk2ajlmNnk5OWZTZ21aQnhiRTRORjgvV2gxV2xOaklQdnJWbkRxODZXbS9yc2tZejNrZW1jb25GTE80NHRjNUg0dXFCaCt2eTFsdGYrR2FmQVcralF0ZWxMT2hQQkM0c0QxMmJoNFlwM01vNlI3bFJSU3Q4RHg5aWo3UG1yOVMwK1RHTDJoL2FqODR1Y2NWenNDY0lRSVZabTRsUHFyTXFZQnA1UkJhNmRSS2F6czB4YVduUC91SHlmNnVyWkRKSW9IVmFXT1V3dGMzYUFybk1YbVU2cFVwMnRTbmhLVHN1VTdwZDkvemhKWndmb0JCUmxkaDdxZDEvL0c3MmZPbDMvWmZxZ3l1UWJpNS83emlreWgrSTFkVTVoMzZsODNaa0dicENIS2UxQ2QxaitWQ0tUVlJ6S01HME1FWGwyMXpKN1NXVGF6SnF2V2JBeFRzaDAzaTB6eUFQM3VFT1ZRUUwzdUkrcVEreTdnNTFsTG1qdHE2Sk53LzVDUEpMeFBUS1BQWlRPK0JPNXV3LzZpMzNramY4bFV4UTI2ZkQ3aTMyWXJSTS9XNVdCbHlTaFovQmdNM2dQOWU1VktZM2ZMVE9xd3I1emFNbzRqc3o3T1BKc2FwbkJIaEZqczNic1YyczZlOTY4c0hpcHp3N1FXKzJmR3A0bmNuOGZuZHZwVHBabWh0SmhEMzFqOFdlZHRhNWpqdnJQY2FqNzhaa3ZXOFdvN3h5UU9henF2QzRHMGxmb3JmNTduTW40V280cTBlWEdLSDIzUngrSm9hL2NCWTNNNmJFcDNYZHI2aVMrSGw5NGlQSHV1M2V4bjc3R2dUdEkvUFJJZzhsd0J6MjdlVFlJM2hndnN0UUQ5emtPMG1NU1ZIZDE2OHh0aXJLQXlucUk1b3RlOE55VWZtVThoZWxyazd0aEdjNTNjYWdQcURVLzExc2NlczRwbWhoRTFvUHZMUW9wMi9kV2NXUW0xSnJiM2JDL1NBTDk5Y3JPczg3MjIxZHJ2cTRpOTdXQmdBYUM1bjdqYTkyZWdmdDhUZmVSM3d5U2NHN1RPb1l6RHQxMUVwa0tGbXRhYzNnSTNJSHhPcVh1cW1lL1E5TVIvc3NGTnFJNVBjZzZDL1hGbjRGcmQ4L3dNRGVxc0c5UWxLM3p3RnQ5Z0Exb0RrL1J0NHVNbW40OVFLYTAzZTZpbEpiQk5ZVE1hOWo0QkQxZDdCa3l2NmRvdnNqREZOVzFOVlA5OFI3cUF6M3dGaWRaSHpKSGZuK3RZaTdyQ2JxRExrcUh1OGkxWlc3emtCakx5RjFRbEE2b2hEN3IyN1JjdmF3S2F4N2xrUm5IL3NiWUJXNlFSTzZ4YTgxbHZCd3E4VVhaVXhtUEtxK212USt6UzI0ZmxCK3pidUJHQTJ0dTkzelNTNkhyOEVEbDVGbjExMHM2TElQTlR0V3Zydy9MU3crMXNWazFOSi84TVdjbjFIZEVjS2tWMWJOSXQxbmc5cEttWjVQUVBIem9yK3RhUkxyVFJaWFJoZVpyL0dFY3h1ZjZiZTNkQjhSNGwzZ1Y2dDBZWFhyd1pNM1grd2FQRGlnZEhnTFBValYwWFdkdERCNUpIYWNtWmhSNmF5YnAxZGp3N0lOZjUwTGEvUzJjeTM1M0trbUxkSHV2Nm1JcWE5czVJWFAySHJqUEVtTlVmMTlHMkNwdXNmQWF3MlJQU3h4cjhxejhkWnpGU280NTZUZXE3cys0dVR3ZHp6R1dtSGJHMWlZbTdmc2pNZDVST2l3amMwYlE0Unl2a3h6M2tYdE1FS2x0VS9HUnVEMTNUbTF2SzV4eGo3bDhsbldpOGplMVY1dU5rWVNweEJZNXU5WTU2aHZjOTNadEhPVkswQXZOV1hXdCs0eG5wc2dscjZ6dE1CMVdvVHZqTXNkSzlzeElrRzRuNGRST2tPN3NybVUxUFZkYU03cVBQT3NxdDY5eDZNNUs2QVlVOWRjblpmOUd4bzZlbHUzYWNwNkJpMzFJMTRrZlg4VkJZbmlUTDh1YzhkQWM5cTI1emNMK29pdHpqdnJyU3NhcWprODdBMzlodzNWOEdseEZlckpIMmZyeFkrNjdzZDkvZnRCKy9MdWo3U0V0NVRiNFhZUGJMY21JcUxUUjl4OGREWmFDY1FRcHlXTDVFV2N3cEhKSjNrTEtjVWRMNFhHTjFaYkp0Vkd2Misxb0tjbXVUanFhZ0VXTXhlVHA5VlVRU2s1cUczM0NCY0taZ0RIV1J2ZmRIeDF0VzFJcUY1ckhQUzRLRXVGNkNTZXBvdml1NVNXbFQ0d1NWTWxWWm1zejhWUmdqak9oZGJRQzU0d1R3UXI1TFl0SmRwUUxNSXlsaUIrTmpLZVMwZzFHQlJaeTMvMjN2RmFvL1ZhNW0yVk1LS3Q0N2ZGbkZ4UEdSYjBteXlkdFZHLzVOL2dJNVdYZ2xqSUU1YUtWUTVFbzZUS2FndGFQSGUxbnJ6SVc0UTJtR0FsVzFCcHpGbzAvVzVHemFJTlJXUkJSVFZnbThGSFU1MFVkMjRuYzhWUjhDOHhaV1NEYzhQRmZNVFdyb1FvbGt6ZVUrKzc1a2pDaEpSZTRzSjYwQytHNDNpRi9HUjVVWUNpd05oSkZpVHZ0RmlwNUJhTzRhQ21WN3h3bE9JWHRsV05MNk1mcnhtMkNhVXJpakJYNHc2TDY3SHZycnR6dEpkQ0dsWkVIK21DUEpDRHBDUTJKUVNLWDhzQ1VkdzVEQW1EaHV4RkZjckdvakZmWmJISndJRFg4WkJOS2NEOXl5NHg0cUZ1eVFidStuc2pCeDYzNWdvYm1yQXU5Z0Zwekk1RkQ2OE4zYzFZR1kvWnVtUU1hNkRPMU5GbVQ1SUJTSjRWZUlzR3hMd0habWt6ekpaR0x5M0VnQjBWUTN3bVVEOWJjb0lnWUhMb0R1b3hadVo3WjI5ZitXdm9qN2YwVzZqMGhHOWJaMlM4S3dMTFZ3ektOOG1oeVgvODNhU2JCS1hobXBaL2wxZkpRLzRkdVZJYjlCWjJRQVFzcVNadGtWMmVuS0wzYlQ4Z3FmdGFISWxURFQ5MUZlbWdpQjlTUm9nUDdjNW5aZlRSbWZ5N1RSUUxWKzFyVTc4TUt1dWpoSVZNeGRYMXZrWWQ5NTJSTjZYVHRyUjZXbVZHRmVrNGxlQ3hUdWd2YzZYNUNXQm1aUFRuUS9weVFnUzRIVUdDKzdpZnhIMzlvUDJRMXBEQWpXNnc2Nk9ibTVpMzdEV3hVNFk3QWxyRXZmM3VQa2NUZmNGNWdCQVdPQU1uQVh0NTE1Y01XN2xrQjJCYkFuT0Nqd0ptOEMvUGIzVDIvSmV6THZ2ZVdYUzdObzc4aUNyR0F2YmRzUjdKb0JDWWxGeXhkTjIzMURTczBKQ3g3eTlwYi9lZ3RBK0EzSU1zWXBDVVhJSVVDSlVBa0dQQWNJN0FsbUVZY2hKaXlRd2ZBTEFJaGxyWktnaTByMGhGNEs3dmRQc3BwV1VDcW52RnRmUlFYck16ckU2bEVxaGdCVkxCTXdKRGZ0cGlEV1BxV1NWV05KWXFydGtjd1VIS2xCcXlubXhjd2ZySkc0QXZNQ2YveWs0Ym1wUGxONHFKVmtZekFCMjFTRFNWY3lGQTNEQnp3TXBkNGdpTVFWa0FraFA5TjhNQ1pUUmw5MC9oMkRqNEFVc1VVb3FRbEJBaG1NbklOSUgrSkNGY1BVdGtHRjNzY2dTMkY4VzNOeTlYSkNFaGNhcVU5WmxpRm4yVzBBaXpEWjhrcWF5RUdLU3gyT0FLUTE4a1RySUR4bWFvVlhKOWVKRjlNL3l2Tlp4WTFRcFR2a0pKSWdlT29KbUU1enNaUGx0UGZLSkJzVGdHUXFEd0NMSHpIU0xSbmVjRnlYQWlDK1prTUFEbU1ybDViVGk0S2tzV1hjNW1mdnlYN0Rid2tHQ0NXdGJIaEdFazdBZUdBWktKZ1VZbnFucnY4RkFWNnQ3Myt2OENCaUFSQUVPRXRMS2tBYW9VQWJGdEx0V1c4cjhUK0R4Y0ZGRGl1QUMvREc5VWlnR01oQzFhUy9xK0s5b1YrMU9id3N4ekllWm5pSm1HeTR1c3BveW9lVW5vcHpuUGlPUlkxK1FVSld1RzFHV3dMa0twYmNCNm0waXpKVW9lK2RxM05iZTNHU0ZsVys0cUpTSEFCYkpoaW5rTVpMbGFBWnJCS0NvNllEUHJsZTl2Yi9PeG1qUVhuSGc2eGJPT29SWTNYOWZJZmRmR1hud0dtVmxTZlhDQ2xWYzlKRnBjVUZqOFpBR1VnQWFRRWNzQnFZeVpMU3dWWFJqd2lQS2V3YWtMVENEbUxiNlZMWUpYMWxMRWloWlJXU3N3VGxIdmxCRW90Wi9XeU9tK0J6OHJpbkF2UURnNnVrRTNDVE5PWkRWNFhMSHU1Nk9JSks0U0t0Q3dIZHFnUGNOR1V2UFN0aHV2cWc1S0xiNDB2WnpGTmltNkFiTWkvblZ6WHY1cCtuRHNaRmdkVzdFZ1cvOFhRc1dyV1R6T214cGtHaDI5SVN3UEExVmJXbUxjN042ZVUzbHFDS09SOEJONjBIY3ZpTisxcWFCUWx4UzBTS3lqNURQY0FKRUxrWitoUTYrMzU3VWE5ajhBWGdibTRvRXNJMFE1bjBRZThhY2E1WGJ1Q0V2YUxqMDhTcE1COTl5Mjcvb204VjYrWHFtUFVEKzBLWnJRZi93a0FBUC8vNWlDT215VVlBQUE9
kind: Secret
metadata:
  labels:
    name: foooo
    owner: helm
    status: deployed
  name: sh.helm.release.v1.foooo.v1
  namespace: appns
type: helm.sh/release.v1
`

// this helm2 release generates the error 'proto: cannot parse invalid wire-format data'
// we shouldn't fail on this error
// TODO: probably this error is caused because of protobuf version. Have to figure out... someday
const helm2ReleaseWithInvalidWireFormat = `
apiVersion: v1
data:
  release: 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
kind: ConfigMap
metadata:
  labels:
    NAME: invalid-wire-data
    OWNER: TILLER
    STATUS: DEPLOYED
    VERSION: "94"
  name: invalid-wire-data.v94
  namespace: kube-system
`

const helm2ReleaseWithDeprecated = `
apiVersion: v1
data:
  release: H4sIAAAAAAAC/+xWvW4kxRbW2tp7V7U32OuAYEkOM8GC5emZcWR1Zmx+LPbH2lkWIYRQTfXp7vJUVzV1To09i+AlyEgIQLwHEQESITEIiSfgAVBV94zX6xXwAEzQ01V96jt/3/m6xfZc+p19sXXrxs7/bn3z61df/ufOt398/eP23Sur3TsnllgaA8o1rUHGuz/8X+yL7aX0g5uTbJpNdl89hHfRNKBq6RlK5+G9MEdvkZEeby2nZ9vTbLLz/ZZ4hbFpjWSksbaVR6JsJRuz892WbPVT9KSdzcEinzu/0LbKFgeUaTdeTufIcioW2hY5nHRHRYMsC8kyFwBWNpjDs/436tEFgLTWsWTtLEU7gMUmtIi8DkMZSZTDYOFsNRDUoorWPhhMx0ZQO+Ic8ELGImTKNQmsZm47WIBWck3rxSgtcxgzEvd7AHOpFmiLfLMBQOiXWuHDFD+q2l1/duo853AwEWLw223xy+0hHGMpg2FYShOQUsWX0mdiCE9qTaAJJHx4+OD+qHS+kcxYQKkNRoNjVEZ6jPZazg0SsIM5QiuJsABt2cHKBQ+bVmVCeGyNVvLIBcs5TIXQjawwVQhbR5qdX+VgK20vBADLKgfiiC4A2mDMqTNarXI4KR86PvVIaLkHOQ3GzFB5ZMrho49FbOSjJXqvC8xhMBBlMObapuhLc6hUikkADGHWotKlRoLzGrlGD3JdQpCdIVDtgiliwsqjZCwE9Hc5sA+YgJ7UmAgFrgSu8RoIOwiEWbI9KcE6BkIGaYseK3Ygou2B7IA0QYUWffQIgbStEvA6t02xN0yOObaumKEKXvPqyFnGC87hs8+T15Le8S60OexPJpNYjZebKdnKuTaadUfjIQAU3rXr+xEc3r+f7j3K4pE1q8fO8dvaIK2IsXmuJj7YQ3robDR4cft9Qp/DtA8l1Sp64FWLORyZQIz+5DRyYcPkfu6iGdrIlCKHUhrCF0a2yyR6+su5XVPvuiEbGkmVShqDHghIo9wP6nquk3KN1tNtnJLmylRHZsaMTHebqjcCSrztRvcKwogN9SW+9DZcK8PLnAmP5IJXSJvmfYAQKEhjVuBRuaZBWySusQNKVF9B0QvB5nQiITswKJcIHNVARjVQzpLSLnRhqdpFPkfdiDQMhD7rpEMacqBtpDEhxVCt6v49U+w2OAtol9o726BlgnPNNRjNbHpGrEPZAwqqju4faKtjV7I4LSsXoHBwLu2VTJ47FmyXLXcz4oxx59pWCd1oG01kcRYoPW+iA4sKiaRf7aX8PTYuZY+ggjcrmHuZalMyerh3Wep7WQ/a6MseqTYkMjf9usEmydt0/+CB7lP8NCD90xPCugJnaFCx86m7gp2JWtAxPFJLlqW2mlfp8e7vW+J2VqNpdGWdx52ft4ZwGmXc2yTX3XZUOQvzoE0R9aSVaiGrKNf9W4BCG+eNgGo0Birj5tBIVrW21R54NJL1EhPBn9uXthBDsFil8OD11mOpL7Do+vzaGxlEmQBn08kYErToU18ykR3PPpmx8yiGcOSaxll4ejSDQnsSWaV5nK5d+CKbP/PjdF1v1NU4XtZLWtrxJVB8d4Y2vcdI7GZ03ordbC4XYjfjphW7X4ghPJU+chxOjt8ikbXenaFikekC5biz8+5MZEtSrsCxGNwU27HeP22J0WgkhjBLvMjjy3H88k8V8e+Hyt9/qBzcePO/vTL9GQAA///hRQQxZgoAAA==
kind: ConfigMap
metadata:
  labels:
    NAME: bar
    OWNER: TILLER
    STATUS: DEPLOYED
  name: bar.v1
  namespace: default
`

const helm3ReleaseWithoutDeprecated = `
apiVersion: v1
data:
  release: SDRzSUFBQUFBQUFDLzN4V1gzT2l2aHIrS2t6T0xYVVJhN2N5ODdzUXRpTHFXdGV1SWh4M3pvUVFJV3RJc2lHbzJQRzdud24wNzdiblhIUk0zbjk1bi9SOW52QUlHQ3d3Y01DTzh3UktZQUxDZGh3NGoyQkhaS24razJKQmVZMVQ0QURic3UwcjYvcks3di9zM2pyZHZtUDNPOWYyemUxMWI5Q0xnUWtvL0gveHR1WFkzYzVYNit2TmRmL202MWVka0dLS1ZSUGFiRW9raVZDRU0rQ0FsY2drVExHQmVDRjBFREJCcWFDcVN1Q0FseU11SmtBNWxFcDNXMkFGVTZpZ1hqOGhTampYZUE1WWxtMVJxOVB0V0IrT0docGpUQXVqcVdUc3VEU21WWUlsd3dxWHdBUlFrUFZMZ1lQZFdNU3JwZHZwM2pRMVZTMzBtVkFJU2hCc1NsOU1RRG5hQTRkVmxKcEE0VUpRcUtzNi8zN3A4Y1g0aGJCTTRyTHMxTENndXNjR0M0ZzJybGdYNnhyWjlKRDg1bGxTckszVUh0VFFwbFZjOStYOStIc0Z3OXREK3Z0dTJ0aSs4ZXdoN0ROVXJNL29LRlM4V2VheFA3S2luM3dhZUc0VmhWMTZUOXdpc1FjazJnUlQxSE5wZEc1OGRScWVLR3JXd3l6d3JBejZnM1A2aldkQkVSK1Mza1RIVjVFOVVFSEdzemJHNWVsNGVieC8yUSt6WU96bXFaK2YzOW04WVRaN2NJL1Jac252eVpETTZtRDYxcWYva0QreTRHalpSLzdxSmhpNWRSekdBZy9mMXdnOGwwVGhYTVpoZi85My9mYnNPVVVzRnBHOXV2SEk4S1AvRlQvRi9rZ2gvMFQvUjl3eDZVMnNUODk0WDZmQjcyWC8vQU11djB4d2dMVFMvOTVIQUhjN3dvaXFnZk40TVFHc0ZDOFJwSVJsMm9rWlRLZ2UreDJrSlRaQkFVOUwzSXhOQ1p5dVpabWdJT3lOeFFRS3lnd3JiN0ZhS1VMSnVSbXZCWllJTXdVekRKeGI2MktDWFVXcG5xcjdBNWFTcExpbEZTbWFpRWNnS2tvWG5CSlVBd2NFdXpsWEM0bEx6QlF3Z2NTQ2wwUnhxWDBzSSt5a0p4cG11c1RscWNhaW92UUJJNG1WSHVCZldpZWFnVzNnTXNaVjAxWFpJdjRiWXM1TDFjNjlYZ0duNWUwVlBrRk43dzdsQ09xaEYxRGxUWFY5bTRxMlN4TjhSTVY0aWg4d3hVaHgyWjRvZURyOHV3dkIwd2VNS2tsVTdYR204RW0xZHRuZXJjY3JwcHI3bGJqa2xVVDRLYS84TEtuRThrQlFlNVZjYTg2dDljSjZqMWFsd2pKWWdOZkFJVUp0L1UrdUIwa01GUWFPa2hVMm42VkE1eXBPc1h5T2JMQ1hLTWNGZk5hUUhhSHY5YU9UWTFxUWpIR0pYMFVqcU4wZjBXWnB4WnRKaFdyWFNtcFh4SGIvZ0lwVmx0bzVUWWhMMHBDV3NhOUZ4RDFxVWtWaFNsRjlQUTFxZHdWOWVnN0c4eTRhdXdmRWxuck40L0JVQm41YUpuYVFKZUhJaXV4Y0pBVXFnL0dFSnBxOG01aTI1TS9lKy8xUkZRLzU3OER2MDlnZVdUQWNWSUdYSDFHeEx1QW1wN0huOXVCbXlRUHZUc3lJdTBpS1V6L3dCMVhjQ2thRElSaTdGQkczaEdHZnpqSmVMVWZ6M2FxMzFIaDB2OThTdTZzUyt6cGI3K2MvQTM4cEVQcytuUldwU0wzYjl0ZW5MQ2tHZGZ5RFZ4RVQ5ZXpZL3NJd3JaTGVoSHFreitOYXgrYnNqZTJjRnRjSGozelBmdGdEbGVpKy9Va2UyYXFMUExlQVdpeVAvTStNelh0b3lQL01pa2tPbS8xU3RmdEJEVU0wbmJMbVRzTm9NeEZKYjMwTzd1amRjdk45T21OdW5kaUNScjBmMDFsQjkzRjRkL0FJcjFLL2UwVEY0STlIK2pheTU0ZllYeDJlQk9haXBZS1JIVzRZZEhWMXRXWC9NaDZhd1hVTS9kaDkrZncxMmJMWFI4d3hHRlpITHZlRVpaMzliZGtoL011aHUyVjd3bExIQ05xc0xYdCtUWjB0TXd3OWFvN1JmaDlzV1Nrd2FzeXlvcmhzVm9aeFpXaGVPOFpXZjBiSUJNb080c1VXdEU3RHlKVVN6dlBHTUJxYXY5bnJmRzNUK1Y5ZXMxNkRmOVlDTzhaQzRoMDV2ZmNtRU8weFM1MzNWc040SXVFSCt6T2FKK1g1Nk5iYy9pVHJPVThqMmJLM1h4Vzlsc0NsZ0ZvY1FJcDNzS0lLWFA0YkFBRC8vMWhJdGNkWkNRQUE=
kind: Secret
metadata:
  labels:
    name: foobar
    owner: helm
    status: deployed
    version: "3"
  name: sh.helm.release.v1.foobar.v3
  namespace: default
type: helm.sh/release.v1
`

const helm3NotDeployed = `
apiVersion: v1
data:
  release: SDRzSUFBQUFBQUFDLzVSWVczUGl1TGIrS3lyUHl6bTFTUnBNNkFsVXpRT213WmlBazBEaTI4bXVVN0lzYkFYWjhsZ3lZTHI2disrU2JBTko5OHl1ZVVqRmx0Zjk4cTBsdm1zWlRMRTIwcmFNTWFaMU5KSnRtVGI2cm0xSndjWC9Semluck1LUk50TDBycTdmZE85dTlNRkw3MjdVKzMzVTdkN2VEL3VEMzM4ZjN2Y0RyYU5SK0E4WklreXhVS1RxaGFPQzVJS3dUQnRwVnNZRnBCUWdsdWFTU090b1hFQlJjbTJrblZYODZHZ29nWVdRNXFaWXdBZ0tLSjh2SG1rZGJZOExYc3ZzM3ZadXV6OXBHb001cGlsUWdzQ1dGZUNoREhHUllZRzUxdEZnVHB5emdMMnVUdkxMU2UrMjkxWEpGRlV1VmNJOHB3UkJKZnBIUjZNTTdiUlJWbExhMFFST2N3cWwxTkgvblUwOEgzNGhXVnhnem04cm1GSnBvM0pGOHowamQxS25RanJkaCs4c0RsT25HK25EQ3VxMERLcEI4VGhmbGRDOTMwZnZNeEo0NjJUMW5CZlFIZXdlaWJFSTA2Z0tQUHMwU1hzME1tYzczMXNuanpHTExYT1FoTzdyVjh0MDdueTNkd2pOVndIZFFZWlM1NFFPOHZ1c0ROTmgxL2ZXZWFnUFRvcG5NbzVoM3lHQnR5Z0RiMDFSTmNqRGFwaTNPcGFwelgzUFBqMlNNWUg2c0F3cTZ3SDFEZXFmMklNMU1hcklQVktrbnNmQ01wTTk2ajlmNnk5OWZTZ21aQnhiRTRORjgvV2gxV2xOaklQdnJWbkRxODZXbS9yc2tZejNrZW1jb25GTE80NHRjNUg0dXFCaCt2eTFsdGYrR2FmQVcralF0ZWxMT2hQQkM0c0QxMmJoNFlwM01vNlI3bFJSU3Q4RHg5aWo3UG1yOVMwK1RHTDJoL2FqODR1Y2NWenNDY0lRSVZabTRsUHFyTXFZQnA1UkJhNmRSS2F6czB4YVduUC91SHlmNnVyWkRKSW9IVmFXT1V3dGMzYUFybk1YbVU2cFVwMnRTbmhLVHN1VTdwZDkvemhKWndmb0JCUmxkaDdxZDEvL0c3MmZPbDMvWmZxZ3l1UWJpNS83emlreWgrSTFkVTVoMzZsODNaa0dicENIS2UxQ2QxaitWQ0tUVlJ6S01HME1FWGwyMXpKN1NXVGF6SnF2V2JBeFRzaDAzaTB6eUFQM3VFT1ZRUUwzdUkrcVEreTdnNTFsTG1qdHE2Sk53LzVDUEpMeFBUS1BQWlRPK0JPNXV3LzZpMzNramY4bFV4UTI2ZkQ3aTMyWXJSTS9XNVdCbHlTaFovQmdNM2dQOWU1VktZM2ZMVE9xd3I1emFNbzRqc3o3T1BKc2FwbkJIaEZqczNic1YyczZlOTY4c0hpcHp3N1FXKzJmR3A0bmNuOGZuZHZwVHBabWh0SmhEMzFqOFdlZHRhNWpqdnJQY2FqNzhaa3ZXOFdvN3h5UU9henF2QzRHMGxmb3JmNTduTW40V280cTBlWEdLSDIzUngrSm9hL2NCWTNNNmJFcDNYZHI2aVMrSGw5NGlQSHV1M2V4bjc3R2dUdEkvUFJJZzhsd0J6MjdlVFlJM2hndnN0UUQ5emtPMG1NU1ZIZDE2OHh0aXJLQXlucUk1b3RlOE55VWZtVThoZWxyazd0aEdjNTNjYWdQcURVLzExc2NlczRwbWhoRTFvUHZMUW9wMi9kV2NXUW0xSnJiM2JDL1NBTDk5Y3JPczg3MjIxZHJ2cTRpOTdXQmdBYUM1bjdqYTkyZWdmdDhUZmVSM3d5U2NHN1RPb1l6RHQxMUVwa0tGbXRhYzNnSTNJSHhPcVh1cW1lL1E5TVIvc3NGTnFJNVBjZzZDL1hGbjRGcmQ4L3dNRGVxc0c5UWxLM3p3RnQ5Z0Exb0RrL1J0NHVNbW40OVFLYTAzZTZpbEpiQk5ZVE1hOWo0QkQxZDdCa3l2NmRvdnNqREZOVzFOVlA5OFI3cUF6M3dGaWRaSHpKSGZuK3RZaTdyQ2JxRExrcUh1OGkxWlc3emtCakx5RjFRbEE2b2hEN3IyN1JjdmF3S2F4N2xrUm5IL3NiWUJXNlFSTzZ4YTgxbHZCd3E4VVhaVXhtUEtxK212USt6UzI0ZmxCK3pidUJHQTJ0dTkzelNTNkhyOEVEbDVGbjExMHM2TElQTlR0V3Zydy9MU3crMXNWazFOSi84TVdjbjFIZEVjS2tWMWJOSXQxbmc5cEttWjVQUVBIem9yK3RhUkxyVFJaWFJoZVpyL0dFY3h1ZjZiZTNkQjhSNGwzZ1Y2dDBZWFhyd1pNM1grd2FQRGlnZEhnTFBValYwWFdkdERCNUpIYWNtWmhSNmF5YnAxZGp3N0lOZjUwTGEvUzJjeTM1M0trbUxkSHV2Nm1JcWE5czVJWFAySHJqUEVtTlVmMTlHMkNwdXNmQWF3MlJQU3h4cjhxejhkWnpGU280NTZUZXE3cys0dVR3ZHp6R1dtSGJHMWlZbTdmc2pNZDVST2l3amMwYlE0Unl2a3h6M2tYdE1FS2x0VS9HUnVEMTNUbTF2SzV4eGo3bDhsbldpOGplMVY1dU5rWVNweEJZNXU5WTU2aHZjOTNadEhPVkswQXZOV1hXdCs0eG5wc2dscjZ6dE1CMVdvVHZqTXNkSzlzeElrRzRuNGRST2tPN3NybVUxUFZkYU03cVBQT3NxdDY5eDZNNUs2QVlVOWRjblpmOUd4bzZlbHUzYWNwNkJpMzFJMTRrZlg4VkJZbmlUTDh1YzhkQWM5cTI1emNMK29pdHpqdnJyU3NhcWprODdBMzlodzNWOEdseEZlckpIMmZyeFkrNjdzZDkvZnRCKy9MdWo3U0V0NVRiNFhZUGJMY21JcUxUUjl4OGREWmFDY1FRcHlXTDVFV2N3cEhKSjNrTEtjVWRMNFhHTjFaYkp0Vkd2Misxb0tjbXVUanFhZ0VXTXhlVHA5VlVRU2s1cUczM0NCY0taZ0RIV1J2ZmRIeDF0VzFJcUY1ckhQUzRLRXVGNkNTZXBvdml1NVNXbFQ0d1NWTWxWWm1zejhWUmdqak9oZGJRQzU0d1R3UXI1TFl0SmRwUUxNSXlsaUIrTmpLZVMwZzFHQlJaeTMvMjN2RmFvL1ZhNW0yVk1LS3Q0N2ZGbkZ4UEdSYjBteXlkdFZHLzVOL2dJNVdYZ2xqSUU1YUtWUTVFbzZUS2FndGFQSGUxbnJ6SVc0UTJtR0FsVzFCcHpGbzAvVzVHemFJTlJXUkJSVFZnbThGSFU1MFVkMjRuYzhWUjhDOHhaV1NEYzhQRmZNVFdyb1FvbGt6ZVUrKzc1a2pDaEpSZTRzSjYwQytHNDNpRi9HUjVVWUNpd05oSkZpVHZ0RmlwNUJhTzRhQ21WN3h3bE9JWHRsV05MNk1mcnhtMkNhVXJpakJYNHc2TDY3SHZycnR6dEpkQ0dsWkVIK21DUEpDRHBDUTJKUVNLWDhzQ1VkdzVEQW1EaHV4RkZjckdvakZmWmJISndJRFg4WkJOS2NEOXl5NHg0cUZ1eVFidStuc2pCeDYzNWdvYm1yQXU5Z0Zwekk1RkQ2OE4zYzFZR1kvWnVtUU1hNkRPMU5GbVQ1SUJTSjRWZUlzR3hMd0habWt6ekpaR0x5M0VnQjBWUTN3bVVEOWJjb0lnWUhMb0R1b3hadVo3WjI5ZitXdm9qN2YwVzZqMGhHOWJaMlM4S3dMTFZ3ektOOG1oeVgvODNhU2JCS1hobXBaL2wxZkpRLzRkdVZJYjlCWjJRQVFzcVNadGtWMmVuS0wzYlQ4Z3FmdGFISWxURFQ5MUZlbWdpQjlTUm9nUDdjNW5aZlRSbWZ5N1RSUUxWKzFyVTc4TUt1dWpoSVZNeGRYMXZrWWQ5NTJSTjZYVHRyUjZXbVZHRmVrNGxlQ3hUdWd2YzZYNUNXQm1aUFRuUS9weVFnUzRIVUdDKzdpZnhIMzlvUDJRMXBEQWpXNnc2Nk9ibTVpMzdEV3hVNFk3QWxyRXZmM3VQa2NUZmNGNWdCQVdPQU1uQVh0NTE1Y01XN2xrQjJCYkFuT0Nqd0ptOEMvUGIzVDIvSmV6THZ2ZVdYUzdObzc4aUNyR0F2YmRzUjdKb0JDWWxGeXhkTjIzMURTczBKQ3g3eTlwYi9lZ3RBK0EzSU1zWXBDVVhJSVVDSlVBa0dQQWNJN0FsbUVZY2hKaXlRd2ZBTEFJaGxyWktnaTByMGhGNEs3dmRQc3BwV1VDcW52RnRmUlFYck16ckU2bEVxaGdCVkxCTXdKRGZ0cGlEV1BxV1NWV05KWXFydGtjd1VIS2xCcXlubXhjd2ZySkc0QXZNQ2YveWs0Ym1wUGxONHFKVmtZekFCMjFTRFNWY3lGQTNEQnp3TXBkNGdpTVFWa0FraFA5TjhNQ1pUUmw5MC9oMkRqNEFVc1VVb3FRbEJBaG1NbklOSUgrSkNGY1BVdGtHRjNzY2dTMkY4VzNOeTlYSkNFaGNhcVU5WmxpRm4yVzBBaXpEWjhrcWF5RUdLU3gyT0FLUTE4a1RySUR4bWFvVlhKOWVKRjlNL3l2Tlp4WTFRcFR2a0pKSWdlT29KbUU1enNaUGx0UGZLSkJzVGdHUXFEd0NMSHpIU0xSbmVjRnlYQWlDK1prTUFEbU1ybDViVGk0S2tzV1hjNW1mdnlYN0Rid2tHQ0NXdGJIaEdFazdBZUdBWktKZ1VZbnFucnY4RkFWNnQ3Myt2OENCaUFSQUVPRXRMS2tBYW9VQWJGdEx0V1c4cjhUK0R4Y0ZGRGl1QUMvREc5VWlnR01oQzFhUy9xK0s5b1YrMU9id3N4ekllWm5pSm1HeTR1c3BveW9lVW5vcHpuUGlPUlkxK1FVSld1RzFHV3dMa0twYmNCNm0waXpKVW9lK2RxM05iZTNHU0ZsVys0cUpTSEFCYkpoaW5rTVpMbGFBWnJCS0NvNllEUHJsZTl2Yi9PeG1qUVhuSGc2eGJPT29SWTNYOWZJZmRmR1hud0dtVmxTZlhDQ2xWYzlKRnBjVUZqOFpBR1VnQWFRRWNzQnFZeVpMU3dWWFJqd2lQS2V3YWtMVENEbUxiNlZMWUpYMWxMRWloWlJXU3N3VGxIdmxCRW90Wi9XeU9tK0J6OHJpbkF2UURnNnVrRTNDVE5PWkRWNFhMSHU1Nk9JSks0U0t0Q3dIZHFnUGNOR1V2UFN0aHV2cWc1S0xiNDB2WnpGTmltNkFiTWkvblZ6WHY1cCtuRHNaRmdkVzdFZ1cvOFhRc1dyV1R6T214cGtHaDI5SVN3UEExVmJXbUxjN042ZVUzbHFDS09SOEJONjBIY3ZpTisxcWFCUWx4UzBTS3lqNURQY0FKRUxrWitoUTYrMzU3VWE5ajhBWGdibTRvRXNJMFE1bjBRZThhY2E1WGJ1Q0V2YUxqMDhTcE1COTl5Mjcvb204VjYrWHFtUFVEKzBLWnJRZi93a0FBUC8vNWlDT215VVlBQUE9
kind: Secret
metadata:
  labels:
    name: foo2
    owner: helm
    status: superseded
  name: sh.helm.release.v1.foo2.v1
  namespace: default
type: helm.sh/release.v1
`

// helm manifestHead can contains duplicated fields. Check it
const releaseWithDoubleFields = `
apiVersion: v1
data:
  release: SDRzSUFBQUFBQUFDLyt4V1hYT2lUQmIrSzFUdkxYRVF4MW1sNnIwUUprR2lZeEtUaUxKNWE2dHBXdWlrUDVqdUJzVlUvdnRXUTB6TVpHN21ldDhMaXo3UCtlaHpudUk4OGd3NFpCaDRZQ3NFNFRtd0FlRmJBYnhuc0NWUzZmOW11S1Npd1Jud2dPdTQ3cG56N2N3WjMvVUgzc0Qxdmc1Nm85SHczOThHMzl4eEFteEE0UjhtWkpoaTNZYTJoa0tTbEpvSURqd1FjYVVocFJZU3JEUkJ3QVpLUTEwcDRJRzNLMTVzZ0Fvb3RXbVhZUTB6cUtFNXY0OEViRkJqcWJxYVRxL2ZjejdkTkxHbW1ES3JMV1J0aGJSbVZZb2x4eG9yWUFOWWt0VmJnZHB0a2ZJZDZmZjYzOXFhdWluTmxiQXNLVUd3TGYxaUF5clFFL0I0UmFrTk5HWWxoYWFxOTUrM0Z0L0FMNFRuRWl2VmF5Q2pwc2QyRkxCWisrV0tyUnJrMGpwOUZIbktWazdtamh2bzBpcHBodkpxK3FPQzhhak9IczluTGZaZDVMZnhrQ08yT3FCZHFaUDFza2pDQzJkekoyWlI0RmVidUUrdmlNOVNkelJEQTU5dURpM2VaUEdlb3ZZOHlhUEF5V0U0UG1UZlJaNndjVDFuaXpxOUVYbm44MFUyWGU2dThxTTl5YU9wWDJSaGNmaUFCWk44ZnV2dk51dWx1Q0tUT2lDVEQ3N1huSHU4OWswL040aXRHRnpuc3c4eHdTVGZzSXRINks3TVhKOThVZUFma3ZXbEMrTUYvZlh1NHk5bEZ6cTVFem1jTGgwMCtYME1Dc2VObWZWVGo4ZGZPT3luOFNWRmp5Sy8rajQ1NWExTVdkWWs2OFhoeHQwWGFMQzRPdVgzSDE3L2pGZnc4cmNOYWtncnN5TFBBRzYzaEJQZEFPLzV4UWF3MGtJaFNJMUdlYzhBYzVoU294eGJTQlcyQVlQN0pXNVhUd0d2N3pnMllJU2ZJRGJRVU9aWUI5ZjM5NXBRY21oWDlCcExoTG1HT1FiZXlIbXh3YmFpMUd6bVZZMmxKQm51bEltd051SVpsQldsMTRJUzFCaUYyaTZFdnBaWVlhNkJEU1F1aFNKYVNPUGpPZUY3b3dvd055VmVYbXRjVjVUZVlpU3hOaUx3dDlIYWR1bmJjVGtYdXUxS2RSUC9PbUlobE82MHc1eUExMG5mR2Q1RG81QTlLaEEwd2xGQ1hiVFZEWnVhZGtjYmZKNktpd3pmWW9xUkZySzdzUlRaNU5jdVNwSGRZbFJKb3B0QWNJMzN1c05seDIwZ0txNWJmaVZXb3BJSXYrYXAzeVVwTEd1Q09pcUZrZTJSODZhY0FhMlV4aks2QnUrQkU0UzYrcitoQjBrTU5RYWVsaFcyajNKcWNyV2dXQjRqMjlrVktqQ0RSeDNlRXZwUmczc0Zwb3prWEVqOExyeFI0OTlzMWtzbldWOVdxUEdkdFBITHhCM1dpTjNubVZ2UWxQZ2tpNmxLUWlQRS9tNFRMK1FtemlocXZzNml4citISVQxRTAwVWZUZjBhOGFVNWl5VGVxeWpNVk9wR2VScGZPQnUzS0ZPR1ZEUzlwR2w0NGNCMVFyc0Z6ai82dzRzcW1ZakhLQnpTeEwxd1lEeXVvcURZZGN0ZDBDVHdCM0M5RkZGd1hzNkpmNTJ5L1RBS3gxWFNDVVU3UXpUMUtTSytndkdRem5OUkxTOFcyL3ZCMHN4ait2MmV1bjJkdWwvejFkUGlMZ3FYSmVJL1puT1dsVmt3NnA0aDVTa2JOOG1OcURhOGJPYTc3Z25qckVvSGx6UWdRNUUwSnJiZ0o5Z2hZMS9yZ1B6SWI5eXhUazNmNFdXeGNYVWZCVDZEUmh4MzR1ZWNMd1pvSW43TzJXVUJXM3VwTzN2Y3dCak5acnpsTk42c0w4dDBzRHBFNS9SOHVmNHhtM08vU2QyU2JnWTNzem1qVDBsOFhnZEVWRm5ZM3lFMi9obVFvWXZjUloyRTkzV1EvL1VYZURGdkE0T2NiSEc3UVdkblp3LzhYOVp0KytKNjFsYUlMNy8vUTM3Zzc5OEJuc1d4M2duNVJIamVleHFwSGhGZjZ2NERmeUk4ODZ5b3kzcmd4KzhSNzRGYmxublQydm9QWEpVWXRaaXNLRmJ0eWJMT0xMUFRiVVFQQ2RhQmxsVm9YWHBIdzdMYTFUNnhUWjdCUE92TEtkZ0YzalVsOXF4cmliZGsvOUdiUXZTRWVlWjlSQzNyZGVrKzRjZjJUVGVmZldhUlBlc3pibG04WWltV25qVnlUcVorWlRXZ1VLbkZPeXYvWjNTY2ZwYjJPL1ZTSlRUS0NMTFJtV3FVeGd5OC9DOEFBUC8vMkpJTHZwd0xBQUE9
kind: Secret
metadata:
  labels:
    modifiedAt: "1654781564"
    name: fooing
    owner: helm
    status: deployed
    version: "1"
  name: sh.helm.release.v1.fooing.v1
  namespace: default
type: helm.sh/release.v1
`
