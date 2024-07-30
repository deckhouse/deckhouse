/*
Copyright 2021 Flant JSC

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

var _ = Describe("ingress-nginx :: hooks :: safe_daemonset_update ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.6", "internal": {}}}`, "")
	f.RegisterCRD("apps.kruise.io", "v1alpha1", "DaemonSet", true)

	Context("Failover pods are ready, update postponed", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  annotations:
    lifecycle.apps.kruise.io/timestamp: "2023-09-28T14:44:21Z"
  labels:
    app: controller
    ingress.deckhouse.io/block-deleting: "true"
    lifecycle.apps.kruise.io/state: "PreparingDelete"
    name: test
  name: controller-test-bw8sc
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: controller
    ingress-nginx-failover: ""
    name: test-failover
  name: controller-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: proxy-failover
    name: test
  name: proxy-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Shouldn't remove blocking label", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-bw8sc")
			Expect(pod.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").Exists()).To(BeTrue())
			Expect(pod.Field("metadata.annotations.ingress\\.deckhouse\\.io/update-postponed-at").Exists()).To(BeTrue())
		})
	})

	Context("Failover pods are ready, pod update was postponed", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  annotations:
    ingress.deckhouse.io/update-postponed-at: "2023-03-24T15:02:56Z"
  labels:
    app: controller
    ingress.deckhouse.io/block-deleting: "true"
    lifecycle.apps.kruise.io/state: "PreparingDelete"
    name: test
  name: controller-test-bw8sc
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: controller
    ingress-nginx-failover: ""
    name: test-failover
  name: controller-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: proxy-failover
    name: test
  name: proxy-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should remove blocking label", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-bw8sc")
			Expect(pod.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").Exists()).To(BeFalse())
		})
	})

	Context("Proxy is not ready", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  annotations:
    lifecycle.apps.kruise.io/timestamp: "2023-09-28T14:44:21Z"
  labels:
    app: controller
    ingress.deckhouse.io/block-deleting: "true"
    lifecycle.apps.kruise.io/state: "PreparingDelete"
    name: test
  name: controller-test-bw8sc
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: controller
    ingress-nginx-failover: ""
    name: test-failover
  name: controller-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: proxy-failover
    name: test
  name: proxy-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 0
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should keep blocking label", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-bw8sc")
			Expect(pod.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").Exists()).To(BeTrue())
		})
	})

	Context("Failover is not ready", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  annotations:
    lifecycle.apps.kruise.io/timestamp: "2023-09-28T14:44:21Z"
  labels:
    app: controller
    ingress.deckhouse.io/block-deleting: "true"
    lifecycle.apps.kruise.io/state: "PreparingDelete"
    name: test
  name: controller-test-bw8sc
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: controller
    ingress-nginx-failover: ""
    name: test-failover
  name: controller-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 1
  updatedNumberScheduled: 0
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: a6fca03cfb274eaa9d1def32dde2bd730ac204baa941ddd88685b47e9b487787
  labels:
    app: proxy-failover
    name: test
  name: proxy-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should keep blocking label", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-bw8sc")
			Expect(pod.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").Exists()).To(BeTrue())
		})
	})

	Context("Checksums are not equal", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  annotations:
    lifecycle.apps.kruise.io/timestamp: "2023-09-28T14:44:21Z"
  labels:
    app: controller
    ingress.deckhouse.io/block-deleting: "true"
    lifecycle.apps.kruise.io/state: "PreparingDelete"
    name: test
  name: controller-test-bw8sc
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: aaaaaa
  labels:
    app: controller
    ingress-nginx-failover: ""
    name: test-failover
  name: controller-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 1
  updatedNumberScheduled: 0
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: bbbb
  labels:
    app: proxy-failover
    name: test
  name: proxy-test-failover
  namespace: d8-ingress-nginx
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should keep blocking label", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-bw8sc")
			Expect(pod.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").Exists()).To(BeTrue())
		})
	})
})
