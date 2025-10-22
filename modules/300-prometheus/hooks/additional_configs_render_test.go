/*
Copyright 2022 Flant JSC

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
1. There are services with label `prometheus.deckhous.io/alertmanager: <prometheus_instance>. Hook must discover them and store to values `prometheus.internal.alertmanagers` in format {"<prometheus_instance>": [{<service_description>}, ...], ...}.
   There is optional annotation `prometheus.deckhouse.io/alertmanager-path-prefix` with default value "/". It must be stored in service description.

*/

package hooks

import (
	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: additional configs", func() {
	const (
		initValuesString       = `{"prometheus": {"internal": {"alertmanagers": {}}}}`
		initConfigValuesString = `{}`
	)
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster has secrets with config", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(configs, 1))
			f.RunHook()
		})
		It("result secret should be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			sec := f.KubernetesResource("Secret", "d8-monitoring", "prometheus-main-additional-configs")
			Expect(sec.Exists()).To(BeTrue())
			Expect(sec.Field("data.alert-managers\\.yaml").String()).To(Equal(""))
			Expect(sec.Field("data.alert-relabels\\.yaml").String()).To(Equal("Ci0gc291cmNlX2xhYmVsczogW25hbWVzcGFjZV0KICByZWdleDogImQ4LS4rfGt1YmUtc3lzdGVtfE5vbmUiCiAgdGFyZ2V0X2xhYmVsOiB0aWVyCiAgcmVwbGFjZW1lbnQ6IGNsdXN0ZXIKLSBzb3VyY2VfbGFiZWxzOiBbbmFtZXNwYWNlLCB0aWVyXQogIHJlZ2V4OiAiXjskIgogIHRhcmdldF9sYWJlbDogdGllcgogIHJlcGxhY2VtZW50OiBjbHVzdGVyCi0gc291cmNlX2xhYmVsczogW3RpZXJdCiAgcmVnZXg6ICJeJCIKICB0YXJnZXRfbGFiZWw6IHRpZXIKICByZXBsYWNlbWVudDogYXBwbGljYXRpb24K"))
			Expect(sec.Field("data.scrapes\\.yaml").String()).To(Equal("Ci0gam9iX25hbWU6IGt1YmUtc3RhdGUtbWV0cmljcy9tYWluCiAgaG9ub3JfbGFiZWxzOiB0cnVlCiAgbWV0cmljc19wYXRoOiAnL21haW4vbWV0cmljcycKICBzY2hlbWU6IGh0dHBzCiAgYmVhcmVyX3Rva2VuX2ZpbGU6IC92YXIvcnVuL3NlY3JldHMva3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC90b2tlbgogIHRsc19jb25maWc6CiAgICBpbnNlY3VyZV9za2lwX3ZlcmlmeTogdHJ1ZQogIHN0YXRpY19jb25maWdzOgogIC0gdGFyZ2V0czogWydrdWJlLXN0YXRlLW1ldHJpY3MuZDgtbW9uaXRvcmluZy5zdmMuY2x1c3Rlci5sb2NhbC46ODA4MCddCiAgcmVsYWJlbF9jb25maWdzOgogIC0gcmVnZXg6IGVuZHBvaW50fG5hbWVzcGFjZXxwb2R8c2VydmljZQogICAgYWN0aW9uOiBsYWJlbGRyb3AKICAtIHRhcmdldF9sYWJlbDogc2NyYXBlX2VuZHBvaW50CiAgICByZXBsYWNlbWVudDogbWFpbgogIC0gdGFyZ2V0X2xhYmVsOiBqb2IKICAgIHJlcGxhY2VtZW50OiBrdWJlLXN0YXRlLW1ldHJpY3MKCi0gam9iX25hbWU6IGt1YmUtc3RhdGUtbWV0cmljcy9zZWxmCiAgaG9ub3JfbGFiZWxzOiB0cnVlCiAgbWV0cmljc19wYXRoOiAnL3NlbGYvbWV0cmljcycKICBzY2hlbWU6IGh0dHBzCiAgdGxzX2NvbmZpZzoKICAgIGluc2VjdXJlX3NraXBfdmVyaWZ5OiB0cnVlCiAgYmVhcmVyX3Rva2VuX2ZpbGU6IC92YXIvcnVuL3NlY3JldHMva3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC90b2tlbgogIHN0YXRpY19jb25maWdzOgogIC0gdGFyZ2V0czogWydrdWJlLXN0YXRlLW1ldHJpY3MuZDgtbW9uaXRvcmluZy5zdmMuY2x1c3Rlci5sb2NhbC46ODA4MCddCiAgcmVsYWJlbF9jb25maWdzOgogIC0gcmVnZXg6IGVuZHBvaW50fG5hbWVzcGFjZXxwb2R8c2VydmljZQogICAgYWN0aW9uOiBsYWJlbGRyb3AKICAtIHRhcmdldF9sYWJlbDogc2NyYXBlX2VuZHBvaW50CiAgICByZXBsYWNlbWVudDogc2VsZgogIC0gdGFyZ2V0X2xhYmVsOiBqb2IKICAgIHJlcGxhY2VtZW50OiBrdWJlLXN0YXRlLW1ldHJpY3MK"))
		})
	})
})

const (
	configs = `
---
apiVersion: v1
data:
  alert-relabels.yaml: Ci0gc291cmNlX2xhYmVsczogW25hbWVzcGFjZV0KICByZWdleDogImQ4LS4rfGt1YmUtc3lzdGVtfE5vbmUiCiAgdGFyZ2V0X2xhYmVsOiB0aWVyCiAgcmVwbGFjZW1lbnQ6IGNsdXN0ZXIKLSBzb3VyY2VfbGFiZWxzOiBbbmFtZXNwYWNlLCB0aWVyXQogIHJlZ2V4OiAiXjskIgogIHRhcmdldF9sYWJlbDogdGllcgogIHJlcGxhY2VtZW50OiBjbHVzdGVyCi0gc291cmNlX2xhYmVsczogW3RpZXJdCiAgcmVnZXg6ICJeJCIKICB0YXJnZXRfbGFiZWw6IHRpZXIKICByZXBsYWNlbWVudDogYXBwbGljYXRpb24=
kind: Secret
metadata:
  labels:
    additional-configs-for-prometheus: main
  name: prometheus-main-additional-configs-alert-relable-tier
  namespace: d8-monitoring
type: Opaque
---
apiVersion: v1
data:
  scrapes.yaml: Ci0gam9iX25hbWU6IGt1YmUtc3RhdGUtbWV0cmljcy9tYWluCiAgaG9ub3JfbGFiZWxzOiB0cnVlCiAgbWV0cmljc19wYXRoOiAnL21haW4vbWV0cmljcycKICBzY2hlbWU6IGh0dHBzCiAgYmVhcmVyX3Rva2VuX2ZpbGU6IC92YXIvcnVuL3NlY3JldHMva3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC90b2tlbgogIHRsc19jb25maWc6CiAgICBpbnNlY3VyZV9za2lwX3ZlcmlmeTogdHJ1ZQogIHN0YXRpY19jb25maWdzOgogIC0gdGFyZ2V0czogWydrdWJlLXN0YXRlLW1ldHJpY3MuZDgtbW9uaXRvcmluZy5zdmMuY2x1c3Rlci5sb2NhbC46ODA4MCddCiAgcmVsYWJlbF9jb25maWdzOgogIC0gcmVnZXg6IGVuZHBvaW50fG5hbWVzcGFjZXxwb2R8c2VydmljZQogICAgYWN0aW9uOiBsYWJlbGRyb3AKICAtIHRhcmdldF9sYWJlbDogc2NyYXBlX2VuZHBvaW50CiAgICByZXBsYWNlbWVudDogbWFpbgogIC0gdGFyZ2V0X2xhYmVsOiBqb2IKICAgIHJlcGxhY2VtZW50OiBrdWJlLXN0YXRlLW1ldHJpY3MKCi0gam9iX25hbWU6IGt1YmUtc3RhdGUtbWV0cmljcy9zZWxmCiAgaG9ub3JfbGFiZWxzOiB0cnVlCiAgbWV0cmljc19wYXRoOiAnL3NlbGYvbWV0cmljcycKICBzY2hlbWU6IGh0dHBzCiAgdGxzX2NvbmZpZzoKICAgIGluc2VjdXJlX3NraXBfdmVyaWZ5OiB0cnVlCiAgYmVhcmVyX3Rva2VuX2ZpbGU6IC92YXIvcnVuL3NlY3JldHMva3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC90b2tlbgogIHN0YXRpY19jb25maWdzOgogIC0gdGFyZ2V0czogWydrdWJlLXN0YXRlLW1ldHJpY3MuZDgtbW9uaXRvcmluZy5zdmMuY2x1c3Rlci5sb2NhbC46ODA4MCddCiAgcmVsYWJlbF9jb25maWdzOgogIC0gcmVnZXg6IGVuZHBvaW50fG5hbWVzcGFjZXxwb2R8c2VydmljZQogICAgYWN0aW9uOiBsYWJlbGRyb3AKICAtIHRhcmdldF9sYWJlbDogc2NyYXBlX2VuZHBvaW50CiAgICByZXBsYWNlbWVudDogc2VsZgogIC0gdGFyZ2V0X2xhYmVsOiBqb2IKICAgIHJlcGxhY2VtZW50OiBrdWJlLXN0YXRlLW1ldHJpY3M=
kind: Secret
metadata:
  labels:
    additional-configs-for-prometheus: main
  name: prometheus-main-additional-configs-kube-state-metrics
  namespace: d8-monitoring
type: Opaque
`
)
