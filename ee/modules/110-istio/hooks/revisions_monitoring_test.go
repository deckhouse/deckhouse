/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"strings"

	"strings"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	nsName  = "ns"
	podName = "pod"
)

const nsTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Name }}
  {{- if or .GlobalRevision .DefiniteRevision }}
  labels:
    {{ if .GlobalRevision }}istio-injection: enabled{{ end }}
    {{ if .DefiniteRevision }}istio.io/rev: "{{ .DefiniteRevision }}"{{ end }}
 {{- end -}}
`

type nsParams struct {
	GlobalRevision   bool
	DefiniteRevision string
	Name             string
}

const podTemplate = `apiVersion: v1
kind: Pod
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    service.istio.io/canonical-name: {{ .Name }}
    {{ if .DisableInjectionLabel }}sidecar.istio.io/inject: "false"{{ end }}
    {{ if .DefiniteRevision }}istio.io/rev: {{ .DefiniteRevision }}{{ end }}
  annotations:
    some-annotation: some-value
    {{ if .CurrentRevision }}
    sidecar.istio.io/status: '{"a":"b", "revision":"{{ .CurrentRevision }}" }'
    {{ end }}
    {{ if .DisableInjectionAnnotation }}sidecar.istio.io/inject: "false"{{ end }}
spec: {}
`

type podParams struct {
	DisableInjectionLabel      bool
	DisableInjectionAnnotation bool
	DefiniteRevision           string
	CurrentRevision            string
	Name                       string
	Namespace                  string
}

type wantedMetric struct {
	Revision        string
	DesiredRevision string
}

func istioNsYAML(ns nsParams) string {
	ns.Name = nsName
	return internal.TemplateToYAML(nsTemplate, ns)
}

func istioPodYAML(pod podParams) string {
	pod.Name = podName
	pod.Namespace = nsName
	return internal.TemplateToYAML(podTemplate, pod)
}

var _ = Describe("Istio hooks :: revisions_monitoring ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}},}`, "")
	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(0))

		})
	})

	DescribeTable("There are different desired and actual revisions", func(objectsYAMLs []string, want *wantedMetric) {
		f.ValuesSet("istio.internal.globalRevision", "v1x42")
		yamlState := strings.Join(objectsYAMLs, "\n---\n")
		f.BindingContexts.Set(f.KubeStateSet(yamlState))

		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
		Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
		m := f.MetricsCollector.CollectedMetrics()

		// the first action should always be "expire"
		Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
			Group:  revisionsMonitoringMetricsGroup,
			Action: "expire",
		}))

		// there are no istio pods or ignored pods in the cluster, hense no metrics
		if yamlState == "" || want == nil {
			Expect(m).To(HaveLen(1))
			return
		}
		Expect(m).To(HaveLen(2))
		Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
			Name:   "d8_istio_pod_revision",
			Group:  revisionsMonitoringMetricsGroup,
			Action: "set",
			Value:  pointer.Float64Ptr(1.0),
			Labels: map[string]string{
				"namespace":        nsName,
				"dataplane_pod":    podName,
				"desired_revision": want.DesiredRevision,
				"revision":         want.Revision,
			},
		}))
	},
		Entry("Empty cluster", []string{}, nil),
		Entry("Pod to ignore with inject=false label",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					DisableInjectionLabel: true,
				}),
			}, nil),
		Entry("Pod to ignore with inject=false annotation",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					DisableInjectionAnnotation: true,
				}),
			}, nil),
		Entry("NS global revision, pod revision is actual",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x42",
				}),
			}, &wantedMetric{
				Revision:        "v1x42",
				DesiredRevision: "v1x42",
			}),
		Entry("NS global revision, pod revision is not actual",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x77",
				}),
			}, &wantedMetric{
				Revision:        "v1x77",
				DesiredRevision: "v1x42",
			}),
		Entry("NS global revision, pod revision is absent (no sidecar)",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{}),
			}, &wantedMetric{
				Revision:        "absent",
				DesiredRevision: "v1x42",
			}),
		Entry("Namespace with definite revision, pod revision is actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x15",
				}),
			}, &wantedMetric{
				Revision:        "v1x15",
				DesiredRevision: "v1x15",
			}),
		Entry("Namespace with definite revision, pod revision is not actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x77",
				}),
			}, &wantedMetric{
				Revision:        "v1x77",
				DesiredRevision: "v1x15",
			}),
		Entry("Namespace with definite revision, pod revision is absent (no sidecar)",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{}),
			}, &wantedMetric{
				Revision:        "absent",
				DesiredRevision: "v1x15",
			}),
		Entry("Namespace with definite revision and pod with definite revision is actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
					CurrentRevision:  "v1x77",
				}),
			}, &wantedMetric{
				Revision:        "v1x77",
				DesiredRevision: "v1x77",
			}),
		Entry("Namespace with definite revision and pod with definite revision is not actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
					CurrentRevision:  "v1x71",
				}),
			}, &wantedMetric{
				Revision:        "v1x71",
				DesiredRevision: "v1x77",
			}),
		Entry("Namespace without labels and pod with definite revision",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
					CurrentRevision:  "v1x77",
				}),
			}, &wantedMetric{
				Revision:        "v1x77",
				DesiredRevision: "v1x77",
			}),
		Entry("Namespace without labels and pod with definite revision but sidecar absent",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
				}),
			}, &wantedMetric{
				Revision:        "absent",
				DesiredRevision: "v1x77",
			}),
		Entry("Pod orphan",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x77",
				}),
			}, &wantedMetric{
				Revision:        "v1x77",
				DesiredRevision: "absent",
			}),
		Entry("Pod without current and desired revisions",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{}),
			}, nil),
	)
})
