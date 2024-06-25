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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/updater"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: update deckhouse image ::", func() {
	f := HookExecutionConfigInit(`{
        "global": {
          "clusterIsBootstrapped": true,
          "modulesImages": {
			"registry": {
				"base": "my.registry.com/deckhouse"
			}
		  }
        },
		"deckhouse": {
              "internal": {},
              "releaseChannel": "Stable",
			  "update": {
				"mode": "Auto",
				"windows": [{"from": "00:00", "to": "23:00"}]
			  }
			}
}`, `{}`)
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "yes")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DeckhouseRelease", false)

	dependency.TestDC.CRClient = cr.NewClientMock(GinkgoT())

	Context("Update out of window", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "8:00", "to": "10:00"}]`))

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusOK,
				}, nil)

			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should keep deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			Expect(rl.Field("status.message").String()).To(Equal("Release is waiting for the update window: 02 Jan 21 08:00 UTC"))
		})
	})

	Context("No update windows configured", func() {
		BeforeEach(func() {
			f.ValuesDelete("deckhouse.update.windows")
			f.ValuesSet("deckhouse.releaseChannel", "Alpha")

			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should upgrade deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.0"))
		})
	})

	Context("Update out of day window", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "8:00", "to": "23:00", "days": ["Mon", "Tue"]}]`))

			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should keep deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
		})
	})

	Context("Update in day window", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "8:00", "to": "23:00", "days": ["Fri", "Sun"]}]`))

			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should upgrade deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.0"))
		})
	})

	Context("Shutdown and evicted pods", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhouseDeployment + deckhousePodsWithShutdown + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should upgrade deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.0"))
		})
	})

	Context("Patch out of update window", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "8:00", "to": "8:01"}]`))

			f.KubeStateSet(deckhousePodYaml + deckhouseReleases + deckhousePatchRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should upgrade deckhouse deployment to patch version", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.1"))
			patchRelease := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1")
			Expect(patchRelease.Field("status.approved").Bool()).To(Equal(true))
		})
	})

	Context("Deckhouse previous release is not ready", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "00:00", "to": "23:59"}]`))

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusInternalServerError,
				}, errors.New("some internal error"))

			f.KubeStateSet(deckhouseDeployment + deckhouseNotReadyPod + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should not upgrade deckhouse version", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
		})
	})

	Context("Manual approval mode is set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.mode", []byte(`"Manual"`))
			f.ValuesDelete("deckhouse.update.windows")

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusOK,
				}, nil)

			f.KubeStateSet(deckhouseDeployment + deckhouseReadyPod + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should not upgrade deckhouse version", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(3))
		})

		Context("After setting manual approve", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.KubeStateSet(deckhouseDeployment + deckhouseReadyPod + manualApprovedReleases)
				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
				f.RunHook()
			})
			It("Must upgrade deckhouse", func() {
				Expect(f).To(ExecuteSuccessfully())
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(Equal("my.registry.com/deckhouse:v1.26.0"))
			})
		})

		Context("Auto deploy Patch release in Manual mode", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.KubeStateSet(deckhouseDeployment + deckhouseReadyPod + deckhouseReleases + deckhousePatchRelease)
				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
				f.RunHook()
			})
			It("Must upgrade deckhouse", func() {
				Expect(f).To(ExecuteSuccessfully())
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(Equal("my.registry.com/deckhouse:v1.25.1"))
			})
		})
	})

	Context("Manual approval mode with canary process", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.mode", []byte(`"Manual"`))
			f.ValuesDelete("deckhouse.update.windows")

			f.KubeStateSet("")
			f.KubeStateSet(deckhousePodYaml + postponedMinorRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should not upgrade deckhouse version", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
		})
		It("Should update release status", func() {
			Expect(f).To(ExecuteSuccessfully())
			release := f.KubernetesGlobalResource("DeckhouseRelease", "v1.36.0")
			Expect(release.Exists()).To(BeTrue())
			Expect(release.Field("status.approved").String()).To(Equal("false"))
			Expect(release.Field("status.message").String()).To(Equal("Waiting for manual approval"))
			Expect(release.Field("status.phase").String()).To(Equal("Pending"))
		})

		Context("After setting manual approve", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.KubeStateSet(deckhousePodYaml + `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.35.0
spec:
  version: "v1.35.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
approved: true
metadata:
  name: v1.36.0
spec:
  version: "v1.36.0"
  applyAfter: "2222-11-11T23:23:23Z"
`)
				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
				f.RunHook()
			})
			It("Must upgrade deckhouse", func() {
				Expect(f).To(ExecuteSuccessfully())
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(Equal("my.registry.com/deckhouse:v1.36.0"))
			})
			It("Should update release status", func() {
				Expect(f).To(ExecuteSuccessfully())
				release := f.KubernetesGlobalResource("DeckhouseRelease", "v1.36.0")
				Expect(release.Exists()).To(BeTrue())
				Expect(release.Field("status.approved").String()).To(Equal("true"))
				Expect(release.Field("status.message").String()).To(Equal(""))
				Expect(release.Field("status.phase").String()).To(Equal("Deployed"))
			})
		})
	})

	Context("DEV: No new deckhouse image", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
				return "sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1", nil
			})
			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should keep deckhouse pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-system", "deckhouse-6f46df5bd7-nk4j7").Exists()).To(BeTrue())
		})
	})

	Context("DEV: Have new deckhouse image", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
				return "sha256:123456", nil
			})
			f.KubeStateSet(deckhousePodYaml)
			f.ValuesDelete("deckhouse.releaseChannel")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should set restart annotation to the deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/restartedAt").String()).To(BeEquivalentTo("2021-01-01T13:30:00Z"))
		})
	})

	Context("Manual mode", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.mode", []byte(`"Manual"`))
			f.ValuesDelete("deckhouse.update.windows")

			f.KubeStateSet(newReleaseState)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should keep deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.2"))
		})

		Context("Second run of the hook in a Manual mode should not change state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
				f.RunHook()
			})

			It("Should keep deckhouse deployment", func() {
				Expect(f).To(ExecuteSuccessfully())
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.2"))
			})
		})
	})

	Context("Single First Release", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + deckhousePatchRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should update deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1").Field("status.phase").String()).To(Equal("Deployed"))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.1"))
		})
	})

	Context("First Release with manual mode", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.mode", []byte(`"Manual"`))
			f.ValuesDelete("deckhouse.update.windows")
			f.ValuesDelete("global.clusterIsBootstrapped")
			f.KubeStateSet(deckhousePodYaml + deckhousePatchRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should update deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1").Field("status.phase").String()).To(Equal("Deployed"))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.1"))
		})
	})

	Context("Few patch releases", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + fewPatchReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should update deckhouse deployment for latest patch", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.31.3").Field("status.phase").String()).To(Equal("Deployed"))
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.31.2").Field("status.phase").String()).To(Equal("Skipped"))
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.31.1").Field("status.phase").String()).To(Equal("Skipped"))
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.32.0").Field("status.phase").String()).To(Equal("Pending"))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.31.3"))
		})
	})

	Context("Pending Manual release on cluster bootstrap", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.mode", []byte(`"Manual"`))
			f.ValuesDelete("deckhouse.update.windows")
			f.ValuesDelete("global.clusterIsBootstrapped")
			f.KubeStateSet(deckhousePodYaml + `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.45.0
spec:
  version: "v1.45.0"
status:
  approved: true
  message: ""
  phase: Deployed
  transitionTime: "2023-04-20T10:10:30Z"
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.46.0
spec:
  version: "v1.46.0"
status:
  approved: false
  message: "Waiting for manual approval"
  phase: Pending
  transitionTime: "2023-05-20T10:10:10Z"
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should not change transition time", func() {
			Expect(f).To(ExecuteSuccessfully())
			rs := f.KubernetesGlobalResource("DeckhouseRelease", "v1.46.0")
			Expect(rs.Field("status.phase").String()).To(Equal("Pending"))
			Expect(rs.Field("status.message").String()).To(Equal("Waiting for manual approval"))
			Expect(rs.Field("status.transitionTime").String()).To(Equal("2023-05-20T10:10:10Z"))
		})
	})

	Context("Forced release", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + forcedRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should update deckhouse even on suspended forced release", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.31.1").Field("status.phase").String()).To(Equal("Deployed"))
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.31.1").Field("metadata.annotations.release\\.deckhouse\\.io/force").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1.31.0").Field("status.phase").String()).To(Equal("Superseded"))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.31.1"))
		})
	})

	Context("Postponed release", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + postponedRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should keep deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			r1250 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.0")
			r1251 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1")
			Expect(r1250.Field("status.phase").String()).To(Equal("Deployed"))
			Expect(r1251.Field("status.phase").String()).To(Equal("Pending"))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
		})

		Context("Release applyAfter time passed", func() {
			BeforeEach(func() {
				f.KubeStateSet(deckhousePodYaml + strings.Replace(postponedRelease, "2222-11-11T23:23:23Z", "2001-11-11T23:23:23Z", 1))

				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
				f.RunHook()
			})

			It("Should upgrade deckhouse deployment", func() {
				Expect(f).To(ExecuteSuccessfully())
				r1250 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.0")
				r1251 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1")
				Expect(r1250.Field("status.phase").String()).To(Equal("Superseded"))
				Expect(r1251.Field("status.phase").String()).To(Equal("Deployed"))
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.1"))
			})
		})
	})

	Context("Suspend release", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + withSuspendedRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should update deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			r1250 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.0")
			r1251 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1")
			r1252 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.2")
			Expect(r1250.Field("status.phase").String()).To(Equal("Superseded"))
			Expect(r1251.Field("status.phase").String()).To(Equal("Suspended"))
			Expect(r1251.Field("metadata.annotations.release\\.deckhouse\\.io/suspended").Exists()).To(BeFalse())
			Expect(r1252.Field("status.phase").String()).To(Equal("Deployed"))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.2"))
		})
	})

	Context("Release with not met requirements and module is enabled", func() {
		BeforeEach(func() {
			requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
				v, _ := getter.Get("global.discovery.kubernetesVersion")
				if v != requirementValue {
					return false, errors.New("min k8s version failed")
				}

				return true, nil
			})
			requirements.SaveValue("global.discovery.kubernetesVersion", "1.16.0")
			f.KubeStateSet(deckhousePodYaml + releaseWithRequirements)
			f.ValuesSet("global.enabledModules", []string{"deckhouse"})
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should not update deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			r130 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.30.0")
			Expect(r130.Field("status.phase").String()).To(Equal("Pending"))
			Expect(r130.Field("status.message").String()).To(Equal(`"k8s" requirement for DeckhouseRelease "1.30.0" not met: min k8s version failed`))
			Expect(f.MetricsCollector.CollectedMetrics()[2].Name).To(Equal("d8_release_blocked"))
			Expect(*f.MetricsCollector.CollectedMetrics()[2].Value).To(Equal(float64(1)))
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
		})

		Context("Release requirements passed", func() {
			BeforeEach(func() {
				requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")
				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
				f.RunHook()
			})

			It("Should update deckhouse deployment", func() {
				Expect(f).To(ExecuteSuccessfully())
				r130 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.30.0")
				Expect(r130.Field("status.phase").String()).To(Equal("Deployed"))
				Expect(r130.Field("status.message").String()).To(Equal(``))
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.30.0"))
			})
		})

	})

	Context("Release with not met requirements and module is disabled", func() {
		BeforeEach(func() {
			requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
				v, _ := getter.Get("global.discovery.kubernetesVersion")
				if v != requirementValue {
					return false, errors.New("min k8s version failed")
				}

				return true, nil
			})
			requirements.SaveValue("global.discovery.kubernetesVersion", "1.16.0")
			f.KubeStateSet(deckhousePodYaml + releaseWithRequirements)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		Context("Release requirements passed", func() {
			It("Should update deckhouse deployment", func() {
				Expect(f).To(ExecuteSuccessfully())
				r130 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.30.0")
				Expect(r130.Field("status.phase").String()).To(Equal("Deployed"))
				Expect(r130.Field("status.message").String()).To(Equal(``))
				dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.30.0"))
			})
		})
	})

	Context("Disruption release", func() {
		BeforeEach(func() {
			f.ValuesSet("deckhouse.update.disruptionApprovalMode", "Manual")
			f.KubeStateSet(deckhousePodYaml + disruptionRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

			var df requirements.DisruptionFunc = func(_ requirements.ValueGetter) (bool, string) {
				return true, "some test reason"
			}
			requirements.RegisterDisruption("testme", df)

			f.RunHook()

		})

		It("Should block the release", func() {
			Expect(f).To(ExecuteSuccessfully())
			r136 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.36.0")
			Expect(r136.Field("status.phase").String()).To(Equal("Pending"))
			Expect(r136.Field("status.message").String()).To(Equal("Release requires disruption approval (`kubectl annotate DeckhouseRelease v1.36.0 release.deckhouse.io/disruption-approved=true`): some test reason"))
		})

		Context("Disruption release approved", func() {
			BeforeEach(func() {
				f.KubeStateSet(deckhousePodYaml + disruptionReleaseApproved)
				f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

				f.RunHook()
			})

			It("Should deploy the release", func() {
				Expect(f).To(ExecuteSuccessfully())
				r136 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.36.0")
				Expect(r136.Field("status.phase").String()).To(Equal("Deployed"))
				Expect(r136.Field("status.message").String()).To(Equal(""))
			})
		})
	})

	Context("Notification: release with notification settings", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		AfterEach(func() {
			defer svr.Close()
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.notification.webhook", []byte(svr.URL))
			f.ValuesSetFromYaml("deckhouse.update.notification.minimalNotificationTime", []byte("1h"))
			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

			f.RunHook()
		})

		It("Should postpone the release", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(httpBody).To(ContainSubstring("New Deckhouse Release 1.26 is available. Release will be applied at: Friday, 01-Jan-21 14:30:00 UTC"))
			Expect(httpBody).To(ContainSubstring(`"version":"1.26"`))
			r126 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			cm := f.KubernetesResource("ConfigMap", "d8-system", "d8-release-data")
			Expect(cm.Field("data.notified").Bool()).To(BeTrue())
			Expect(r126.Field("status.phase").String()).To(Equal("Pending"))
			Expect(r126.Field("spec.applyAfter").String()).To(Equal("2021-01-01T14:30:00Z"))
			Expect(r126.Field("metadata.annotations.release\\.deckhouse\\.io/notification-time-shift").Exists()).To(BeTrue())
		})
	})

	Context("Notification: after met conditions", func() {
		BeforeEach(func() {
			changedState := `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.26.0
spec:
  applyAfter: "2019-01-01T01:01:00Z"
  version: v1.26.0
status:
  phase: Pending
---
apiVersion: v1
data:
  isUpdating: "false"
  notified: "true"
kind: ConfigMap
metadata:
  name: d8-release-data
  namespace: d8-system
`
			f.KubeStateSet(deckhousePodYaml + changedState)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Release should be deployed", func() {
			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "d8-system", "d8-release-data")
			r126 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			Expect(r126.Field("status.phase").String()).To(Equal("Deployed"))
			Expect(cm.Field("data.isUpdating").Bool()).To(BeTrue())
			Expect(cm.Field("data.notified").Bool()).To(BeFalse())
		})
	})

	Context("Update: Release is deployed", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.26.0
spec:
  version: "v1.26.0"
status:
  phase: Deployed
---
apiVersion: v1
data:
  isUpdating: "true"
  notified: "false"
kind: ConfigMap
metadata:
  name: d8-release-data
  namespace: d8-system
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("IsUpdating flag should be reset", func() {
			Expect(f).To(ExecuteSuccessfully())
			r126 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			Expect(r126.Field("status.phase").String()).To(Equal("Deployed"))
			cm := f.KubernetesResource("ConfigMap", "d8-system", "d8-release-data")
			Expect(cm.Field("data.isUpdating").Bool()).To(BeFalse())
			Expect(cm.Field("data.notified").Bool()).To(BeFalse())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Group).To(Equal("d8_updating"))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Action).To(Equal("expire"))
		})
	})

	Context("Notification: release applyAfter time is after notification period", func() {
		var httpBody string
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			httpBody = string(data)
		}))
		AfterEach(func() {
			defer svr.Close()
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.notification.webhook", []byte(svr.URL))
			f.ValuesDelete("deckhouse.update.windows")
			f.ValuesSetFromYaml("deckhouse.update.notification.minimalNotificationTime", []byte("4h10m"))
			f.KubeStateSet(deckhousePodYaml + postponedMinorRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

			f.RunHook()
		})

		It("Should not change postpone time", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(httpBody).To(ContainSubstring("New Deckhouse Release 1.36 is available. Release will be applied at: Monday, 11-Nov-22 23:23:23 UTC"))
			Expect(httpBody).To(ContainSubstring(`"version":"1.36"`))
			r136 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.36.0")
			cm := f.KubernetesResource("ConfigMap", "d8-system", "d8-release-data")
			Expect(cm.Field("data.notified").Bool()).To(BeTrue())
			Expect(r136.Field("status.phase").String()).To(Equal("Pending"))
			Expect(r136.Field("spec.applyAfter").String()).To(Equal("2222-11-11T23:23:23Z"))
			Expect(r136.Field("metadata.annotations.release\\.deckhouse\\.io/notification-time-shift").Exists()).To(BeFalse())
		})
	})

	Context("Notification: basic auth", func() {
		var (
			username string
			password string
		)
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			username, password, _ = r.BasicAuth()
		}))
		AfterEach(func() {
			defer svr.Close()
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.notification.webhook", []byte(svr.URL))
			f.ValuesSet("deckhouse.update.notification.auth", updater.Auth{Basic: &updater.BasicAuth{Username: "user", Password: "pass"}})
			f.ValuesDelete("deckhouse.update.windows")
			f.KubeStateSet(deckhousePodYaml + postponedMinorRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

			f.RunHook()
		})

		It("Should have basic auth in headers", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(username).To(Equal("user"))
			Expect(password).To(Equal("pass"))
		})
	})

	Context("Notification: bearer token auth", func() {
		var (
			headerValue string
		)
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			headerValue = r.Header.Get("Authorization")
		}))
		AfterEach(func() {
			defer svr.Close()
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.notification.webhook", []byte(svr.URL))
			f.ValuesSet("deckhouse.update.notification.auth", updater.Auth{Token: pointer.String("the_token")})
			f.ValuesDelete("deckhouse.update.windows")
			f.KubeStateSet(deckhousePodYaml + postponedMinorRelease)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

			f.RunHook()
		})

		It("Should have bearer token in headers", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(headerValue).To(Equal("Bearer the_token"))
		})
	})

	Context("Update minimal notification time without configuring notification webhook", func() {
		svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		AfterEach(func() {
			defer svr.Close()
		})

		BeforeEach(func() {
			f.ValuesSet("deckhouse.update.disruptionApprovalMode", "Manual")
			f.ValuesSetFromYaml("deckhouse.update.notification.minimalNotificationTime", []byte("2h"))
			f.KubeStateSet(deckhousePodYaml + deckhouseReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))

			f.RunHook()
		})

		It("Should postpone the release without webhook notification", func() {
			Expect(f).To(ExecuteSuccessfully())
			r126 := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			cm := f.KubernetesResource("ConfigMap", "d8-system", "d8-release-data")
			Expect(cm.Field("data.notified").Bool()).To(BeTrue())
			Expect(r126.Field("status.phase").String()).To(Equal("Pending"))
			Expect(r126.Field("spec.applyAfter").String()).To(Equal("2021-01-01T15:30:00Z"))
			Expect(r126.Field("metadata.annotations.release\\.deckhouse\\.io/notification-time-shift").Exists()).To(BeTrue())
		})
	})

	Context("release with apply-now annotation out of window", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "8:00", "to": "10:00"}]`))

			f.KubeStateSet(deckhousePodYaml + appliedNowReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should upgrade deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.0"))

			release := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			Expect(release.Field(`metadata.annotations.release\.deckhouse\.io/apply-now`).Exists()).To(BeFalse())
		})
	})

	Context("Deckhouse previous release is not ready", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "00:00", "to": "23:59"}]`))

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusInternalServerError,
				}, errors.New("some internal error"))

			f.KubeStateSet(deckhouseDeployment + deckhouseNotReadyPod + appliedNowReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should not upgrade deckhouse version", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.0"))
		})
	})

	Context("Manual approval mode is set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("deckhouse.update.mode", []byte(`"Manual"`))
			f.ValuesDelete("deckhouse.update.windows")

			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					StatusCode: http.StatusOK,
				}, nil)

			f.KubeStateSet(deckhouseDeployment + deckhouseReadyPod + appliedNowReleases)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})
		It("Should upgrade deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.26.0"))

			release := f.KubernetesGlobalResource("DeckhouseRelease", "v1.26.0")
			Expect(release.Field(`metadata.annotations.release\.deckhouse\.io/apply-now`).Exists()).To(BeFalse())
		})
	})

	Context("applied now postponed release", func() {
		BeforeEach(func() {
			f.KubeStateSet(deckhousePodYaml + `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.1
  annotations:
    release.deckhouse.io/apply-now: "true"
spec:
  version: "v1.25.1"
  applyAfter: "2222-11-11T23:23:23Z"
status:
  phase: Pending
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should upgrade deckhouse deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(dep.Field("spec.template.spec.containers").Array()[0].Get("image").String()).To(BeEquivalentTo("my.registry.com/deckhouse:v1.25.1"))

			release := f.KubernetesGlobalResource("DeckhouseRelease", "v1.25.1")
			Expect(release.Field(`metadata.annotations.release\.deckhouse\.io/apply-now`).Exists()).To(BeFalse())
		})
	})
})

const (
	deckhouseReadyPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-6f46df5bd7-nk4j7
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:v1.2.3
status:
  containerStatuses:
    - containerID: containerd://9990d3eccb8657d0bfe755672308831b6d0fab7f3aac553487c60bf0f076b2e3
      imageID: dev-registry.deckhouse.io/sys/deckhouse-oss/dev@sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1
      ready: true
`

	deckhouseNotReadyPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-6f46df5bd7-nk4j7
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:test-me
status:
  containerStatuses:
    - containerID: containerd://9990d3eccb8657d0bfe755672308831b6d0fab7f3aac553487c60bf0f076b2e3
      imageID: dev-registry.deckhouse.io/sys/deckhouse-oss/dev@sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1
      ready: false
`

	deckhouseDeployment = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
        - name: deckhouse
          image: my.registry.com/deckhouse:v1.25.0
`

	deckhousePodYaml = deckhouseReadyPod + deckhouseDeployment

	deckhouseReleases = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.26.0
spec:
  version: "v1.26.0"
`

	manualApprovedReleases = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.26.0
spec:
  version: "v1.26.0"
approved: true
`

	appliedNowReleases = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.26.0
  annotations:
    release.deckhouse.io/apply-now: "true"
spec:
  version: "v1.26.0"
`

	deckhousePatchRelease = `

---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.1
spec:
  version: "v1.25.1"
`

	newReleaseState = `
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.26.2
spec:
  version: v1.26.2
status:
  approved: true
  phase: Deployed
  transitionTime: "2021-12-08T08:34:01.292180321Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  name: v1.27.0
spec:
  version: v1.27.0
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-6f46df5bd7-nk4j7
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:1.26.2
status:
  containerStatuses:
    - containerID: containerd://9990d3eccb8657d0bfe755672308831b6d0fab7f3aac553487c60bf0f076b2e3
      imageID: dev-registry.deckhouse.io/sys/deckhouse-oss/dev@sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1
      ready: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
        - name: deckhouse
          image: my.registry.com/deckhouse:v1.26.2
`

	withSuspendedRelease = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.1
  annotations:
    release.deckhouse.io/suspended: "true"
spec:
  version: "v1.25.1"
status:
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.2
spec:
  version: "v1.25.2"
status:
  phase: Pending
`

	postponedRelease = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.25.1
spec:
  version: "v1.25.1"
  applyAfter: "2222-11-11T23:23:23Z"
status:
  phase: Pending
`

	postponedMinorRelease = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.35.0
spec:
  version: "v1.35.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.36.0
spec:
  version: "v1.36.0"
  applyAfter: "2222-11-11T23:23:23Z"
status:
  phase: Pending
`

	releaseWithRequirements = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.29.0
spec:
  version: "v1.29.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.30.0
spec:
  version: "v1.30.0"
  requirements:
    k8s: "1.19.0"
status:
  phase: Pending
`
	fewPatchReleases = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.31.0
spec:
  version: "v1.31.0"
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.31.1
spec:
  version: "v1.31.1"
status:
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.31.2
spec:
  version: "v1.31.2"
status:
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.31.3
spec:
  version: "v1.31.3"
status:
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.32.0
spec:
  version: "v1.32.0"
status:
  phase: Pending
`
	forcedRelease = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.31.0
spec:
  version: "v1.31.0"
status:
  phase: Pending
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.31.1
  annotations:
    release.deckhouse.io/force: "true"
spec:
  version: "v1.31.1"
status:
  phase: Suspended
`

	disruptionRelease = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.36.0
spec:
  version: "v1.36.0"
  disruptions:
    - testme
status:
  phase: Pending
`
	disruptionReleaseApproved = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1.36.0
  annotations:
    release.deckhouse.io/disruption-approved: "true"
spec:
  version: "v1.36.0"
  disruptions:
    - testme
status:
  phase: Pending
`

	deckhousePodsWithShutdown = `
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-6f46df5bd7-nk4j1
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:v1.2.3
status:
  message: 'The node was low on resource: memory. Container xxx was using 1014836Ki, which exceeds its request of 300Mi.'
  phase: Failed
  reason: Evicted
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-6f46df5bd7-nk4j2
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:v1.2.3
status:
  phase: Running
  containerStatuses:
    - containerID: containerd://9990d3eccb8657d0bfe755672308831b6d0fab7f3aac553487c60bf0f076b2e3
      imageID: dev-registry.deckhouse.io/sys/deckhouse-oss/dev@sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1
      ready: true
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-6f46df5bd7-nk4j3
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:v1.2.3
status:
  message: 'Node is shutting, evicting pods'
  phase: Failed
  reason: Shutdown
`
)
