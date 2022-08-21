/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: revisions_monitoring ::", func() {
	f := HookExecutionConfigInit(`
{
  "istio":{"internal":{}},
  "global":{"modulesImages":{"tags":{"istio":{
    "operatorV123": "deadbeef",
    "proxyv2V1x10x1": "xxx-v1x10x1",
    "proxyv2V1x15": "xxx-v1x15",
    "proxyv2V1x42": "xxx-v1x42"
  }}}}
}
`, "")

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

	Context("Empty cluster and revisions are discovered", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalRevision", "v1x42")
			f.ValuesSet("istio.internal.revisionsToInstall", []string{})
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  revisionsMonitoringMetricsGroup,
				Action: "expire",
			}))
		})
	})

	Context("There are different desired and actual revisions", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalRevision", "v1x42")
			f.ValuesSet("istio.internal.revisionsToInstall", []string{"v1x15", "v1x10x1", "v1x42"})

			namespacesYAML := `
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    istio-injection: enabled
  name: ns-global
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    istio.io/rev: v1x15
  name: ns-rev1x15
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-nodesired
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    istio.io/rev: v1xexotic
  name: ns-exotic
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    istio.io/rev: v1x10x1
  name: ns-v1x10x1
`
			podsYAML := `---
apiVersion: v1
kind: Pod
metadata:
  name: pod-to-ignore
  namespace: ns-global
  labels:
    sidecar.istio.io/inject: false
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x13"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x13
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-definite-revision-installed
  namespace: ns-nodesired
  labels:
    istio.io/rev: v1x15
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x15"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x15
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-definite-revision-not-installed
  namespace: ns-nodesired
  labels:
    istio.io/rev: v1xexotic
    service.istio.io/canonical-name: qqq
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1xexotic
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-global-revision-actual
  namespace: ns-global
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x42"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x42
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-global-revision-not-actual
  namespace: ns-global
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x15"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x15
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-definite-ns-revision-actual
  namespace: ns-rev1x15
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x15"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x15
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-definite-ns-revision-not-actual
  namespace: ns-rev1x15
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x42"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x42
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-orphan
  namespace: ns-nodesired
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x15"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x13
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-minor-version-is-actual
  namespace: ns-global
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x42"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x42
  - name: bbb
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-minor-version-mismatch
  namespace: ns-global
  labels:
    service.istio.io/canonical-name: qqq
  annotations:
    sidecar.istio.io/status: '{"a":"b","revision":"v1x42"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x42-stale
  - name: bbb
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-regular-v1x10x1
  namespace: ns-v1x10x1
  labels:
    service.istio.io/canonical-name: qqq
    istio.io/rev: v1x10x1
  annotations:
    sidecar.istio.io/status: '{"a":"b"}'
spec:
  containers:
  - name: aaa
  - name: istio-proxy
    image: registry.deckhouse.io/deckhouse/ee:xxx-v1x10x1
  - name: bbb
`

			f.BindingContexts.Set(f.KubeStateSet(namespacesYAML + podsYAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(11))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  revisionsMonitoringMetricsGroup,
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"namespace":        "ns-global",
					"dataplane_pod":    "pod-global-revision-actual",
					"desired_revision": "v1x42",
					"revision":         "v1x42",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"desired_revision": "v1x42",
					"revision":         "v1x15",
					"namespace":        "ns-global",
					"dataplane_pod":    "pod-global-revision-not-actual",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"desired_revision": "v1x42",
					"revision":         "v1x42",
					"namespace":        "ns-global",
					"dataplane_pod":    "pod-minor-version-is-actual",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"desired_revision": "v1x42",
					"revision":         "v1x42",
					"namespace":        "ns-global",
					"dataplane_pod":    "pod-minor-version-mismatch",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"namespace":        "ns-nodesired",
					"dataplane_pod":    "pod-definite-revision-installed",
					"desired_revision": "v1x15",
					"revision":         "v1x15",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"namespace":        "ns-nodesired",
					"dataplane_pod":    "pod-definite-revision-not-installed",
					"desired_revision": "v1xexotic",
					"revision":         "unknown",
				},
			}))
			Expect(m[7]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"dataplane_pod":    "pod-orphan",
					"desired_revision": "unknown",
					"revision":         "v1x15",
					"namespace":        "ns-nodesired",
				},
			}))
			Expect(m[8]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"dataplane_pod":    "pod-definite-ns-revision-actual",
					"desired_revision": "v1x15",
					"revision":         "v1x15",
					"namespace":        "ns-rev1x15",
				},
			}))

			Expect(m[9]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"desired_revision": "v1x15",
					"revision":         "v1x42",
					"namespace":        "ns-rev1x15",
					"dataplane_pod":    "pod-definite-ns-revision-not-actual",
				},
			}))
			Expect(m[10]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_pod_revision",
				Group:  revisionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"namespace":        "ns-v1x10x1",
					"dataplane_pod":    "pod-regular-v1x10x1",
					"desired_revision": "v1x10x1",
					"revision":         "v1x10x1",
				},
			}))
		})
	})
})
