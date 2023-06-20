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
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.applicationNamespaces").Array()).To(BeEmpty())
		})
	})

	Context("Application namespaces with labels but pods with labels", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), &v1core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-pod-0"}}, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), &v1core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-pod-1"}}, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), &v1core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-pod-2"}}, metav1.CreateOptions{})
			f.BindingContexts.Set(f.KubeStateSet(`
---
# pod without any revision
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
  namespace: ns-pod-1
spec: {}
---
# pod with global revision
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
  namespace: ns-pod-1
  labels:
    sidecar.istio.io/inject: "true"
spec: {}
---
# pod with definite revision
apiVersion: v1
kind: Pod
metadata:
  name: pod-2
  namespace: ns-pod-2
  labels:
    istio.io/rev: v1x11
spec: {}
`))

			f.RunHook()
		})

		It("Should count all pods namespaces properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()).To(Equal([]string{"ns-pod-1", "ns-pod-2"}))
		})
	})

	Context("Application namespaces with and without discard-metrics labels", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.KubeStateSet(`
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
---
`))

			f.RunHook()
		})

		It("Should count all pods namespaces properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()).To(Equal([]string{"ns-0", "ns-1", "ns-2", "ns-3"}))
			Expect(f.ValuesGet("istio.internal.applicationNamespacesToMonitor").AsStringSlice()).To(Equal([]string{"ns-0", "ns-2"}))
		})
	})

	Context("Application namespaces with labels and IstioOperator", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
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
`))
			f.RunHook()
		})
		It("Should count all namespaces properly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()).To(Equal([]string{"d8-ns6", "d8-ns7", "kube-ns8", "kube-ns9", "ns1", "ns2", "ns3", "ns4", "ns5"}))
		})
	})
})
