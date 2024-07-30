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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const istioOperatorTemplate = `
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  labels:
    app: istiod
    istio.io/rev: {{ .Revision }}
  name: {{ .Revision }}
  namespace: d8-istio
spec:
  revision: {{ .Revision }}
status:
  componentStatus:
    Pilot:
{{- if eq .PilotStatus "ERROR" }}
  {{- if .ValidationError }}
      error: 'failed to update resource with server-side apply for obj EnvoyFilter/d8-istio/stats-filter-{{ .Revision }}:
        Internal error occurred: failed calling webhook "validation.istio.io": Post
        "https://istiod.d8-istio.svc:443/validate?timeout=10s": dial tcp 10.222.166.108:443:
        i/o timeout, failed to update resource with server-side apply for obj EnvoyFilter/d8-istio/stats-filter-{{ .Revision }}:
        Internal error occurred: failed calling webhook "validation.istio.io": Post
        "https://istiod.d8-istio.svc:443/validate?timeout=10s": context deadline exceeded'
  {{ else }}
      error: 'other error'
  {{- end }}
{{- end }}
      status: {{ .PilotStatus }}
  status: {{ .Status }}
`

const podOperatorTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "{{ .TimestampRFC3339 }}"
  labels:
    app: operator
    revision: {{ .Revision }}
  name: {{ .Name }}
  namespace: d8-istio
spec: {}
status:
  phase: {{ .Phase }}
  startTime: "{{ .TimestampRFC3339 }}"
`

type istioOperatorParams struct {
	Revision        string
	Status          string
	PilotStatus     string
	ValidationError bool
}

type IstioOperatorPodParams struct {
	Name             string
	Revision         string
	Phase            string
	Timestamp        time.Time
	TimestampRFC3339 string
}

func istioOperatorYaml(iop istioOperatorParams) string {
	return lib.TemplateToYAML(istioOperatorTemplate, iop)
}

func istioOperatorPodYaml(pod IstioOperatorPodParams) string {
	if len(pod.TimestampRFC3339) == 0 {
		pod.TimestampRFC3339 = pod.Timestamp.Format(time.RFC3339)
	}
	return lib.TemplateToYAML(podOperatorTemplate, pod)
}

var _ = Describe("Istio hooks :: hack iop reconciling ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)

	Context("Empty cluster and minimal settings.", func() {
		BeforeEach(func() {
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Iop: healty. Pod: running.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:    "v1x88",
				PilotStatus: "HEALTHY",
				Status:      "HEALTHY",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "healthy-operator",
				Phase:     "Running",
				Revision:  "v1x88",
				Timestamp: time.Now(),
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "healthy-operator").Exists()).To(BeTrue())
		})
	})

	Context("Iop: error, pilot: healthy. Pod: running", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:    "v1x88",
				PilotStatus: "HEALTHY",
				Status:      "ERROR",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Phase:     "Running",
				Revision:  "v1x88",
				Timestamp: time.Now(),
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeTrue())
		})
	})

	Context("Iop: error, pilot: error (other). Pod: running.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:        "v1x88",
				PilotStatus:     "ERROR",
				ValidationError: false,
				Status:          "ERROR",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Phase:     "Running",
				Revision:  "v1x88",
				Timestamp: time.Now(),
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeTrue())
		})
	})

	Context("Iop: error, pilot: error (validating webhook). Pod: running, created 6 min ago.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:        "v1x33",
				PilotStatus:     "ERROR",
				ValidationError: true,
				Status:          "ERROR",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Phase:     "Running",
				Revision:  "v1x33",
				Timestamp: time.Now().Add(-time.Minute * 6),
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeFalse())
		})
	})

	Context("Iop: error, pilot: error (validating webhook). Pod: pending, created 6 min ago.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:        "v1x33",
				PilotStatus:     "ERROR",
				ValidationError: true,
				Status:          "ERROR",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Phase:     "Pending",
				Revision:  "v1x33",
				Timestamp: time.Now().Add(-time.Minute * 6),
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeTrue())
		})
	})

	Context("Iop: error, pilot: error (validating webhook). Pod: running, created less than 5 min ago.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:        "v1x33",
				PilotStatus:     "ERROR",
				ValidationError: true,
				Status:          "ERROR",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Revision:  "v1x33",
				Phase:     "Running",
				Timestamp: time.Now(),
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeTrue())
		})
	})

	Context("Iops with mixed statuses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioOperatorYaml(istioOperatorParams{
				Revision:    "v1x88",
				PilotStatus: "HEALTHY",
				Status:      "HEALTHY",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "healthy-operator",
				Revision:  "v1x88",
				Phase:     "Running",
				Timestamp: time.Now(),
			}) + istioOperatorYaml(istioOperatorParams{
				Revision:        "v1x33",
				PilotStatus:     "ERROR",
				ValidationError: true,
				Status:          "ERROR",
			}) + istioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Phase:     "Running",
				Revision:  "v1x33",
				Timestamp: time.Now().Add(-time.Minute * 6),
			}),
			))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "healthy-operator").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeFalse())
		})
	})
})
