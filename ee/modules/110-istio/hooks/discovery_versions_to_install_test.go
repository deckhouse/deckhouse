/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
    "1.1": {"fullVersion": "1.1.1"},
    "1.2": {"fullVersion": "1.2.11"}
  }
globalVersion: "1.2" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.versionsToInstall").String()).To(MatchJSON(`["1.2"]`))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.2"))

			assertTelemetryMetrics(f, "1.2.11")
		})
	})

	Context("No globalVersion in CM and globalVersion was previously discovered", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.10": {"fullVersion": "1.10.10"},
    "1.3": {"fullVersion": "1.3.1"},
    "1.4": {"fullVersion": "1.4.3"},
    "1.42": {"fullVersion": "1.42.42"}
  }
  globalVersion: "1.42"
globalVersion: "1.4" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.RunHook()
		})
		It("Previously discovered value 1.42 must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.42"}))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.42"))

			assertTelemetryMetrics(f, "1.42.42")
		})
	})

	Context("No globalVersion in CM and the global service without annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.10": {"fullVersion": "1.10.10"},
    "1.3": {"fullVersion": "1.3.1"},
    "1.4": {"fullVersion": "1.4.3"},
  }
globalVersion: "1.4" # default version "from openapi/values.yaml"
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
    "1.10": {"fullVersion": "1.10.10"},
    "1.3": {"fullVersion": "1.3.1"},
    "1.4": {"fullVersion": "1.4.3"},
  }
globalVersion: "1.4" # default version "from openapi/values.yaml"
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
    istio.deckhouse.io/global-version: "1.3"
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
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.3"}))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.3"))

			assertTelemetryMetrics(f, "1.3.1")
		})
	})

	Context("globalVersion in CM and the global service with annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.10": {"fullVersion": "1.10.10"},
    "1.2": {"fullVersion": "1.2.4"},
    "1.3": {"fullVersion": "1.3.1"},
    "1.4": {"fullVersion": "1.4.3"},
  }
globalVersion: "1.4" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "1.2")

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    istio.deckhouse.io/global-version: "1.3"
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
			Expect(f.ValuesGet("istio.internal.versionsToInstall").AsStringSlice()).To(Equal([]string{"1.2"}))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.2"))

			assertTelemetryMetrics(f, "1.2.4")
		})
	})

	Context("Unsupported versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  versionMap: {
    "1.1": {"fullVersion": "1.1.5"},
    "1.2": {"fullVersion": "1.2.3"},
    "1.3": {"fullVersion": "1.3.11"},
  }
globalVersion: "1.3" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "2.0")
			f.ConfigValuesSet("istio.additionalVersions", []string{"1.1", "1.3", "2.7", "2.8", "2.9"})
			f.RunHook()
		})
		It("Should return errors", func() {
			Expect(f).ToNot(ExecuteSuccessfully())

			Expect(f.GoHookError).To(MatchError("unsupported versions: [2.0,2.7,2.8,2.9]"))

			assertNoMetrics(f)
		})
	})
})
