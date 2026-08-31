/*
Copyright 2023 Flant JSC

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_versions_to_install ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")

	assertNoMetrics := func(f *HookExecutionConfig) {
		metrics := f.MetricsCollector.CollectedMetrics()
		Expect(metrics).To(HaveLen(0))
	}

	assertTelemetryMetrics := func(f *HookExecutionConfig, version string) {
		metrics := f.MetricsCollector.CollectedMetrics()
		Expect(metrics).To(HaveLen(1))
		Expect(metrics[0].Name).To(Equal("d8_telemetry_istio_control_plane_full_version"))
		Expect(metrics[0].Labels["full_version"]).To(Equal(version))
	}

	Context("Empty cluster and no settings", func() {
		BeforeEach(func() {
			values := `
internal:
  versionMap: {
    "1.25": {"fullVersion": "1.25.2"},
    "1.27": {"fullVersion": "1.27.9"}
  }
globalVersion: "1.27" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LoggerOutput.Contents()).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.versionsToInstall").String()).To(MatchJSON(`["1.27"]`))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.27"))

			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("1.27"))

			assertTelemetryMetrics(f, "1.27.9")
		})
	})

	Context("No globalVersion in CM and globalVersion was previously discovered", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.25": {"fullVersion": "1.25.2"},
    "1.27": {"fullVersion": "1.27.9"},
    "1.29": {"fullVersion": "1.29.6"}
  }
  globalVersion: "1.29"
globalVersion: "1.27" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.RunHook()
		})
		It("Previously discovered value 1.29 must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.29"}))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.29"))

			assertTelemetryMetrics(f, "1.29.6")
		})
	})

	Context("No globalVersion in CM and the global service without annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.25": {"fullVersion": "1.25.2"},
    "1.27": {"fullVersion": "1.27.9"},
    "1.29": {"fullVersion": "1.29.6"},
  }
globalVersion: "1.27" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations: {}
  name: istiod
  namespace: d8-istio
spec: {}
`), &service)
			Expect(err).To(BeNil())

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services(service.GetNamespace()).
				Create(context.TODO(), &service, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})
		It("Hook must fail with error", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("can't find istio.deckhouse.io/global-version annotation for istiod global Service d8-istio/istiod"))

			assertNoMetrics(f)
		})
	})

	Context("No globalVersion in CM and the global service with annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.25": {"fullVersion": "1.25.2"},
    "1.27": {"fullVersion": "1.27.9"},
    "1.29": {"fullVersion": "1.29.6"},
  }
globalVersion: "1.27" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    istio.deckhouse.io/global-version: "1.29"
  name: istiod
  namespace: d8-istio
spec: {}
`), &service)
			Expect(err).To(BeNil())

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services(service.GetNamespace()).
				Create(context.TODO(), &service, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})
		It("globalVersion should be gathered from the Service", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.29"}))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.29"))

			assertTelemetryMetrics(f, "1.29.6")
		})
	})

	Context("globalVersion in CM and the global service with annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.25": {"fullVersion": "1.25.2"},
    "1.27": {"fullVersion": "1.27.9"},
    "1.29": {"fullVersion": "1.29.6"}
  }
globalVersion: "1.27" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "1.25")

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    istio.deckhouse.io/global-version: "1.29"
  name: istiod
  namespace: d8-istio
spec: {}
`), &service)
			Expect(err).To(BeNil())

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services(service.GetNamespace()).
				Create(context.TODO(), &service, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})
		It("globalVersion should be gathered from CM", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.25"}))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.25"))

			assertTelemetryMetrics(f, "1.25.2")
		})
	})

	Context("Migration from 1.21 to 1.25 publishes only the configured supported version", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			values := `
internal:
  versionMap: {
    "1.25": {"fullVersion": "1.25.2"},
    "1.27": {"fullVersion": "1.27.9"},
    "1.29": {"fullVersion": "1.29.6"}
  }
globalVersion: "1.25"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "1.25")
			f.RunHook()
		})

		It("does not treat the retired revision as installed for release requirements", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.25"}))
			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("1.25"))
		})
	})

	Context("Operator-free versions set minimal version requirement", func() {
		BeforeEach(func() {
			f.KubeStateSet("")

			values := `
internal:
  versionMap: {
    "1.27": {"fullVersion": "1.27.9"},
    "1.29": {"fullVersion": "1.29.6"}
  }
globalVersion: "1.27"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "1.27")
			f.ConfigValuesSet("istio.additionalVersions", []string{"1.29"})
			f.RunHook()
		})

		It("Should publish minimal version from versionsToInstall", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.27", "1.29"}))

			value, exists := requirements.GetValue(minVersionValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("1.27"))

			installedVersions, exists := requirements.GetValue(installedVersionsValuesKey)
			Expect(exists).To(BeTrue())
			Expect(installedVersions).To(BeEquivalentTo([]string{"1.27", "1.29"}))
		})
	})
})
