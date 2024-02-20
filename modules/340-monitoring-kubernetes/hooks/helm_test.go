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
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	helmreleases "github.com/deckhouse/deckhouse/modules/340-monitoring-kubernetes/hooks/internal"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("helm :: hooks :: deprecated_versions ::", func() {
	f := HookExecutionConfigInit(`{"global" : {"discovery": {"kubernetesVersion": "1.22.3"}}}`, "")
	helmReleasesInterval = helmreleases.IntervalImmediately

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

// helm manifest can contains duplicated fields. Check it
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
