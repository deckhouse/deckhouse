/*
Copyright 2026 Flant JSC

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

var _ = Describe("Istio hooks :: reserved UID monitoring ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully; only expire metric group", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Group).To(Equal("d8_istio_reserved_uid"))
		})
	})

	Context("Pod with app container running as UID 1337 and istio-proxy present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(podAppUser1337WithProxy))
			f.RunHook()
		})

		It("Should emit metric for the app container", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Name).To(Equal("d8_istio_pod_container_reserved_uid"))
			Expect(m[1].Labels).To(BeEquivalentTo(map[string]string{
				"namespace": "default",
				"pod":       "app-pod",
				"container": "app",
			}))
		})
	})

	Context("Pod with only istio-proxy running as UID 1337, app has normal UID", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(podOnlyProxyUser1337))
			f.RunHook()
		})

		It("Should not emit per-container metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
		})
	})

	Context("Pod without istio canonical-name label, app running as UID 1337", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(podUser1337NoProxy))
			f.RunHook()
		})

		It("Should not emit per-container metrics since pod is not managed by Istio", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
		})
	})

	Context("Pod with pod-level runAsUser 1337 and istio-proxy present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(podPodLevelUser1337WithProxy))
			f.RunHook()
		})

		It("Should emit metric for app container inheriting pod-level UID", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Name).To(Equal("d8_istio_pod_container_reserved_uid"))
			Expect(m[1].Labels).To(BeEquivalentTo(map[string]string{
				"namespace": "default",
				"pod":       "pod-level-uid",
				"container": "app",
			}))
		})
	})

	Context("Pod with pod-level runAsUser 1337, container overrides to different UID, istio-proxy present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(podOverriddenUserWithProxy))
			f.RunHook()
		})

		It("Should not emit per-container metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
		})
	})

	Context("Multiple app containers with UID 1337 in one pod with istio-proxy", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(podMultipleContainers1337))
			f.RunHook()
		})

		It("Should emit metrics for each non-istio-proxy container", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(m[1].Name).To(Equal("d8_istio_pod_container_reserved_uid"))
			Expect(m[2].Name).To(Equal("d8_istio_pod_container_reserved_uid"))
		})
	})

})

const (
	podAppUser1337WithProxy = `
---
apiVersion: v1
kind: Pod
metadata:
  name: app-pod
  namespace: default
  labels:
    service.istio.io/canonical-name: app
spec:
  containers:
  - name: app
    image: app:latest
    securityContext:
      runAsUser: 1337
  - name: istio-proxy
    image: istio/proxyv2:latest
    securityContext:
      runAsUser: 1337
`

	podOnlyProxyUser1337 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: proxy-only
  namespace: default
  labels:
    service.istio.io/canonical-name: proxy-only
spec:
  containers:
  - name: app
    image: app:latest
    securityContext:
      runAsUser: 1000
  - name: istio-proxy
    image: istio/proxyv2:latest
    securityContext:
      runAsUser: 1337
`

	podUser1337NoProxy = `
---
apiVersion: v1
kind: Pod
metadata:
  name: no-proxy-pod
  namespace: default
spec:
  containers:
  - name: app
    image: app:latest
    securityContext:
      runAsUser: 1337
`

	podPodLevelUser1337WithProxy = `
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-level-uid
  namespace: default
  labels:
    service.istio.io/canonical-name: pod-level-uid
spec:
  securityContext:
    runAsUser: 1337
  containers:
  - name: app
    image: app:latest
  - name: istio-proxy
    image: istio/proxyv2:latest
`

	podOverriddenUserWithProxy = `
---
apiVersion: v1
kind: Pod
metadata:
  name: overridden-pod
  namespace: default
  labels:
    service.istio.io/canonical-name: overridden-pod
spec:
  securityContext:
    runAsUser: 1337
  containers:
  - name: app
    image: app:latest
    securityContext:
      runAsUser: 1000
  - name: istio-proxy
    image: istio/proxyv2:latest
`

	podMultipleContainers1337 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: multi-container-pod
  namespace: default
  labels:
    service.istio.io/canonical-name: multi-container-pod
spec:
  containers:
  - name: app
    image: app:latest
    securityContext:
      runAsUser: 1337
  - name: sidecar
    image: sidecar:latest
    securityContext:
      runAsUser: 1337
  - name: istio-proxy
    image: istio/proxyv2:latest
    securityContext:
      runAsUser: 1337
`

)
