/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
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
  phase: Running
  startTime: "{{ .TimestampRFC3339 }}"
`

type istioOperatorParams struct {
	Revision string
	Status   string
}

type IstioOperatorPodParams struct {
	Name             string
	Revision         string
	Timestamp        time.Time
	TimestampRFC3339 string
}

func IstioOperatorYaml(iop istioOperatorParams) string {
	return internal.TemplateToYAML(istioOperatorTemplate, iop)
}

func IstioOperatorPodYaml(pod IstioOperatorPodParams) string {
	if len(pod.TimestampRFC3339) == 0 {
		pod.TimestampRFC3339 = pod.Timestamp.Format(time.RFC3339)
	}
	return internal.TemplateToYAML(podOperatorTemplate, pod)
}

const (
	healthyOperatorPodName = "healthy-operator"
	erroredOperatorPodName = "errored-operator"
)

var _ = FDescribe("Istio hooks :: handle_operator_bootstrap ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
	Context("Istio operator without error status", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(IstioOperatorYaml(istioOperatorParams{
				Revision: "v1x88",
				Status:   "HEALTHY",
			}) + IstioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "healthy-operator",
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
	Context("Istio operator with error status, operator's pod created 6 min ago", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(IstioOperatorYaml(istioOperatorParams{
				Revision: "v1x33",
				Status:   "ERROR",
			}) + IstioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Revision:  "v1x33",
				Timestamp: time.Now().Add(time.Minute * 6),
			})))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeFalse())
		})
	})
	Context("Istio operator with error status, operator's pod created less than 5 min ago", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(IstioOperatorYaml(istioOperatorParams{
				Revision: "v1x33",
				Status:   "ERROR",
			}) + IstioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Revision:  "v1x33",
				Timestamp: time.Now(),
			})))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeTrue())
		})
	})
	Context("Istio operators with mixed statuses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(IstioOperatorYaml(istioOperatorParams{
				Revision: "v1x88",
				Status:   "HEALTHY",
			}) + IstioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "healthy-operator",
				Revision:  "v1x88",
				Timestamp: time.Now(),
			}) + IstioOperatorYaml(istioOperatorParams{
				Revision: "v1x33",
				Status:   "ERROR",
			}) + IstioOperatorPodYaml(IstioOperatorPodParams{
				Name:      "errored-operator",
				Revision:  "v1x33",
				Timestamp: time.Now().Add(time.Minute * 6),
			}),
			))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-istio", "errored-operator").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-istio", "healthy-operator").Exists()).To(BeTrue())
		})
	})
})
