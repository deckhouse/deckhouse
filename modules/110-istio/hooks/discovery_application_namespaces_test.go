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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_application_namespaces ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LoggerOutput.Contents()).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.applicationNamespaces").Array()).To(BeEmpty())
		})
	})

	Context("Application namespaces with labels but pods with labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(applicationNamespacesWithLabeledPods))
			f.RunHook()
		})

		It("Should count all pods namespaces properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()).To(Equal([]string{"ns-istio-defined-revision-a", "ns-istio-defined-revision-b-with-injection", "ns-istio-injection-enabled", "ns-without-labels-b"}))
		})
	})

	Context("Application namespaces with and without discard-metrics labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(applicationNamespacesWithDiscardMetrics))
			f.RunHook()
		})

		It("Should count all pods namespaces properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()).To(Equal([]string{"ns-0", "ns-1", "ns-2", "ns-3"}))
		})
		It("Should count all pods namespaces to monitor properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespacesToMonitor").AsStringSlice()).To(Equal([]string{"ns-0", "ns-2"}))
		})
	})

	Context("Application namespaces with labels and IstioOperator", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(applicationNamespacesRevisionAndPrefixes))
			f.RunHook()
		})
		It("Should count all namespaces properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()).To(Equal([]string{"d8-ns6", "d8-ns7", "kube-ns8", "kube-ns9", "ns1", "ns2", "ns3", "ns4", "ns5"}))
		})
	})
})

const (
	applicationNamespacesWithLabeledPods = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-istio-injection-enabled
  labels:
    istio-injection: enabled
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-istio-defined-revision-a
  labels:
    istio.io/rev: v1x13
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-istio-defined-revision-b-with-injection
  labels:
    istio.io/rev: v1x14
    istio-injection: enabled
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-without-labels-a
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-without-labels-b
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-without-labels-c
---
# pod without any revision
apiVersion: v1
kind: Pod
metadata:
  name: pod-0
  namespace: ns-without-labels-a
spec: {}
---
# pod with global revision
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
  namespace: ns-without-labels-b
  labels:
    sidecar.istio.io/inject: "true"
spec: {}
---
# pod with definite revision on empty ns
apiVersion: v1
kind: Pod
metadata:
  name: pod-2
  namespace: ns-without-labels-c
  labels:
    istio.io/rev: v1x11
spec: {}
---
# pod with definite revision poiniting on revisioned ns
apiVersion: v1
kind: Pod
metadata:
  name: pod-3
  namespace: ns-istio-defined-revision-b-with-injection
  labels:
    istio.io/rev: v1x12
spec: {}
`

	applicationNamespacesWithDiscardMetrics = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-0
  labels: {}
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-1
  labels:
    istio.deckhouse.io/discard-metrics: "true"
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-2
  labels:
    istio-injection: enabled
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns-3
  labels:
    istio-injection: enabled
    istio.deckhouse.io/discard-metrics: "true"
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-0
  namespace: ns-0
  labels:
    sidecar.istio.io/inject: "true"
spec: {}
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
  namespace: ns-1
  labels:
    sidecar.istio.io/inject: "true"
spec: {}
`

	applicationNamespacesRevisionAndPrefixes = `
---
# regular ns
apiVersion: v1
kind: Namespace
metadata:
  name: ns0
  labels: {}
---
# ns with global revision
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
  labels:
    istio-injection: enabled
---
# ns with global revision
apiVersion: v1
kind: Namespace
metadata:
  name: ns2
  labels:
    istio-injection: enabled
---
# ns with definite revision
apiVersion: v1
kind: Namespace
metadata:
  name: ns3
  labels:
    istio.io/rev: v1x7x4
---
# ns with definite revision
apiVersion: v1
kind: Namespace
metadata:
  name: ns4
  labels:
    istio.io/rev: v1x5x0
---
# ns with definite revision
apiVersion: v1
kind: Namespace
metadata:
  name: ns5
  labels:
    istio.io/rev: v1x7x4
---
# ns with definite revision with d8 prefix
apiVersion: v1
kind: Namespace
metadata:
  name: d8-ns6
  labels:
    istio.io/rev: v1x8x0
---
# ns with global revision with d8 prefix
apiVersion: v1
kind: Namespace
metadata:
  name: d8-ns7
  labels:
    istio-injection: enabled
---
# ns with definite revision with kube prefix
apiVersion: v1
kind: Namespace
metadata:
  name: kube-ns8
  labels:
    istio.io/rev: v1x9x0
---
# ns with global revision with kube prefix
apiVersion: v1
kind: Namespace
metadata:
  name: kube-ns9
  labels:
    istio-injection: enabled
---
# ns with deletionTimestamp
apiVersion: v1
kind: Namespace
metadata:
  name: kube-ns10
  annotations:
    deletionTimestamp: "2020-10-22T21:30:34Z"
  labels:
    istio-injection: enabled
`
)
