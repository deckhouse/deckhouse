/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
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
    {{ if .InjectionLabel }}sidecar.istio.io/inject: "{{ .InjectionLabelValue }}"{{ end }}
    {{ if .DefiniteRevision }}istio.io/rev: {{ .DefiniteRevision }}{{ end }}
  annotations:
    some-annotation: some-value
    {{ if .Version }}
    istio.deckhouse.io/version: '{{ .Version }}'
    {{ end }}
    {{ if .CurrentRevision }}
    sidecar.istio.io/status: '{"a":"b", "revision":"{{ .CurrentRevision }}" }'
    {{ end }}
    {{ if .DisableInjectionAnnotation }}sidecar.istio.io/inject: "false"{{ end }}
spec: {}
`

type podParams struct {
	InjectionLabel             bool
	InjectionLabelValue        bool
	DisableInjectionAnnotation bool
	DefiniteRevision           string
	CurrentRevision            string
	Version                    string
	Name                       string
	Namespace                  string
}

type wantedMetric struct {
	Revision           string
	DesiredRevision    string
	Version            string
	DesiredVersion     string
	FullVersion        string
	DesiredFullVersion string
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

	var hookInitValues = `
{  "istio":
  { "internal":
    { "versionMap":
      {
         "1.15": { revision: "v1x15", fullVersion: "1.15.15" },
         "1.42": { revision: "v1x42", fullVersion: "1.42.42" },
         "1.71": { revision: "v1x71", fullVersion: "1.71.71" },
         "1.77": { revision: "v1x77", fullVersion: "1.77.77" },
         "1.155": { revision: "v1x155", fullVersion: "1.155.155" }
      }
    }
  }
}
`

	f := HookExecutionConfigInit(hookInitValues, "")
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
		f.ValuesSet("istio.internal.globalVersion", "1.42")
		yamlState := strings.Join(objectsYAMLs, "\n---\n")
		f.BindingContexts.Set(f.KubeStateSet(yamlState))

		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
		Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
		m := f.MetricsCollector.CollectedMetrics()

		// the first action should always be "expire"
		Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
			Group:  metadataExporterMetricsGroup,
			Action: "expire",
		}))

		// there are no istio pods or ignored pods in the cluster, hense no metrics
		if yamlState == "" || want == nil {
			Expect(m).To(HaveLen(1))
			return
		}
		Expect(m).To(HaveLen(2))
		Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
			Name:   istioPodMetadataMetricName,
			Group:  metadataExporterMetricsGroup,
			Action: "set",
			Value:  pointer.Float64Ptr(1.0),
			Labels: map[string]string{
				"namespace":            nsName,
				"dataplane_pod":        podName,
				"desired_revision":     want.DesiredRevision,
				"revision":             want.Revision,
				"version":              want.Version,
				"desired_version":      want.DesiredVersion,
				"full_version":         want.FullVersion,
				"desired_full_version": want.DesiredFullVersion,
			},
		}))
	},

		// Checks for normal behavior, everything with revision is ok!
		Entry("Empty cluster", []string{}, nil),
		Entry("NS with global revision, Pod to ignore with inject=false label",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					InjectionLabel:      true,
					InjectionLabelValue: false,
				}),
			}, nil),
		Entry("NS with definite revision, but revision is absent in revisionFullVersionMap",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x00",
				}),
				istioPodYAML(podParams{
					InjectionLabel:      true,
					InjectionLabelValue: true,
					CurrentRevision:     "v1x00",
					Version:             "", // annotation is absent
				}),
			}, &wantedMetric{
				Revision:           "v1x00",
				DesiredRevision:    "v1x00",
				Version:            "unknown",
				DesiredVersion:     "unknown",
				FullVersion:        "unknown",
				DesiredFullVersion: "unknown",
			}),
		Entry("NS without any revisions, pod with inject=true label",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: false,
				}),
				istioPodYAML(podParams{
					InjectionLabel:      true,
					InjectionLabelValue: true,
					CurrentRevision:     "v1x42",
					Version:             "1.42.42",
				}),
			}, &wantedMetric{
				Revision:           "v1x42",
				DesiredRevision:    "v1x42",
				Version:            "1.42",
				DesiredVersion:     "1.42",
				FullVersion:        "1.42.42",
				DesiredFullVersion: "1.42.42",
			}),
		Entry("NS with global revision, pod with inject=true label",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					InjectionLabel:      true,
					InjectionLabelValue: true,
					CurrentRevision:     "v1x42",
					Version:             "1.42.42",
				}),
			}, &wantedMetric{
				Revision:           "v1x42",
				DesiredRevision:    "v1x42",
				Version:            "1.42",
				DesiredVersion:     "1.42",
				FullVersion:        "1.42.42",
				DesiredFullVersion: "1.42.42",
			}),
		Entry("NS with definite revision, pod with inject=true label",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					InjectionLabel:      true,
					InjectionLabelValue: true,
					CurrentRevision:     "v1x15",
					Version:             "1.15.15",
				}),
			}, &wantedMetric{
				Revision:           "v1x15",
				DesiredRevision:    "v1x15",
				Version:            "1.15",
				DesiredVersion:     "1.15",
				FullVersion:        "1.15.15",
				DesiredFullVersion: "1.15.15",
			}),
		Entry("NS without any revisions, pod with istio.io/rev label",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: false,
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x15",
					CurrentRevision:  "v1x15",
					Version:          "1.15.15",
				}),
			}, &wantedMetric{
				Revision:           "v1x15",
				DesiredRevision:    "v1x15",
				Version:            "1.15",
				DesiredVersion:     "1.15",
				FullVersion:        "1.15.15",
				DesiredFullVersion: "1.15.15",
			}),
		Entry("NS with global revision, pod with istio.io/rev label",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x15",
					CurrentRevision:  "v1x15",
					Version:          "1.15.15",
				}),
			}, &wantedMetric{
				Revision:           "v1x15",
				DesiredRevision:    "v1x15",
				Version:            "1.15",
				DesiredVersion:     "1.15",
				FullVersion:        "1.15.15",
				DesiredFullVersion: "1.15.15",
			}),
		Entry("NS with definite revision, pod with inject=true label",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x155",
					CurrentRevision:  "v1x155",
					Version:          "1.155.155",
				}),
			}, &wantedMetric{
				Revision:           "v1x155",
				DesiredRevision:    "v1x155",
				Version:            "1.155",
				DesiredVersion:     "1.155",
				FullVersion:        "1.155.155",
				DesiredFullVersion: "1.155.155",
			}),
		Entry("NS with global revision, Pod to ignore with inject=false annotation",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					DisableInjectionAnnotation: true,
				}),
			}, nil),
		Entry("NS with definite revision, Pod to ignore with inject=false annotation",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					DisableInjectionAnnotation: true,
				}),
			}, nil),
		Entry("NS with global revision, Pod revision is actual",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x42",
					Version:         "1.42.42",
				}),
			}, &wantedMetric{
				Revision:           "v1x42",
				DesiredRevision:    "v1x42",
				Version:            "1.42",
				DesiredVersion:     "1.42",
				FullVersion:        "1.42.42",
				DesiredFullVersion: "1.42.42",
			}),
		Entry("Namespace with definite revision, pod revision is actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x15",
					Version:         "1.15.15",
				}),
			}, &wantedMetric{
				Revision:           "v1x15",
				DesiredRevision:    "v1x15",
				Version:            "1.15",
				DesiredVersion:     "1.15",
				FullVersion:        "1.15.15",
				DesiredFullVersion: "1.15.15",
			}),

		// Checks for revision inconsistencies
		Entry("NS global revision, pod revision is not actual",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x77",
					Version:         "1.77.77",
				}),
			}, &wantedMetric{
				Revision:           "v1x77",
				DesiredRevision:    "v1x42",
				Version:            "1.77",
				DesiredVersion:     "1.42",
				FullVersion:        "1.77.77",
				DesiredFullVersion: "1.42.42",
			}),
		Entry("NS global revision, pod revision is absent (no sidecar)",
			[]string{
				istioNsYAML(nsParams{
					GlobalRevision: true,
				}),
				istioPodYAML(podParams{}),
			}, &wantedMetric{
				Revision:           "absent",
				DesiredRevision:    "v1x42",
				Version:            "absent",
				DesiredVersion:     "1.42",
				FullVersion:        "absent",
				DesiredFullVersion: "1.42.42",
			}),
		Entry("Namespace with definite revision, pod revision is not actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x77",
					Version:         "1.77.77",
				}),
			}, &wantedMetric{
				Revision:           "v1x77",
				DesiredRevision:    "v1x15",
				Version:            "1.77",
				DesiredVersion:     "1.15",
				FullVersion:        "1.77.77",
				DesiredFullVersion: "1.15.15",
			}),
		Entry("Namespace with definite revision, pod revision is absent (no sidecar)",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{}),
			}, &wantedMetric{
				Revision:           "absent",
				DesiredRevision:    "v1x15",
				Version:            "absent",
				DesiredVersion:     "1.15",
				FullVersion:        "absent",
				DesiredFullVersion: "1.15.15",
			}),
		Entry("Namespace with definite revision and pod with definite revision is actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
					CurrentRevision:  "v1x77",
					Version:          "1.77.77",
				}),
			}, &wantedMetric{
				Revision:           "v1x77",
				DesiredRevision:    "v1x77",
				Version:            "1.77",
				DesiredVersion:     "1.77",
				FullVersion:        "1.77.77",
				DesiredFullVersion: "1.77.77",
			}),
		Entry("Namespace with definite revision and pod with definite revision is not actual",
			[]string{
				istioNsYAML(nsParams{
					DefiniteRevision: "v1x15",
				}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
					CurrentRevision:  "v1x71",
					Version:          "1.71.71",
				}),
			}, &wantedMetric{
				Revision:           "v1x71",
				DesiredRevision:    "v1x77",
				Version:            "1.71",
				DesiredVersion:     "1.77",
				FullVersion:        "1.71.71",
				DesiredFullVersion: "1.77.77",
			}),
		Entry("Namespace without labels and pod with definite revision",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
					CurrentRevision:  "v1x77",
					Version:          "1.77.77",
				}),
			}, &wantedMetric{
				Revision:           "v1x77",
				DesiredRevision:    "v1x77",
				Version:            "1.77",
				DesiredVersion:     "1.77",
				FullVersion:        "1.77.77",
				DesiredFullVersion: "1.77.77",
			}),
		Entry("Namespace without labels and pod with definite revision but sidecar absent",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{
					DefiniteRevision: "v1x77",
				}),
			}, &wantedMetric{
				Revision:           "absent",
				DesiredRevision:    "v1x77",
				Version:            "absent",
				DesiredVersion:     "1.77",
				FullVersion:        "absent",
				DesiredFullVersion: "1.77.77",
			}),
		Entry("Pod orphan",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{
					CurrentRevision: "v1x77",
					Version:         "1.77.77",
				}),
			}, &wantedMetric{
				Revision:           "v1x77",
				DesiredRevision:    "absent",
				Version:            "1.77",
				DesiredVersion:     "unknown",
				FullVersion:        "1.77.77",
				DesiredFullVersion: "unknown",
			}),
		Entry("Pod without current and desired revisions",
			[]string{
				istioNsYAML(nsParams{}),
				istioPodYAML(podParams{}),
			}, nil),
	)
})
