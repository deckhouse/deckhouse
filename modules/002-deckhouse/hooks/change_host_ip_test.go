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

var _ = Describe("Modules :: deckhouse :: hooks :: change host ip ::", func() {
	f := HookExecutionConfigInit(`{deckhouse: {internal: {currentReleaseImageName: "test"}}}`, `{}`)

	Context("With Deckhouse pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-test
  namespace: d8-system
  labels:
    app: deckhouse
status:
  hostIP: 1.2.3.4
`, 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-system", "deckhouse-test")
			Expect(pod.Exists()).To(BeTrue())
			Expect(pod.Field(`metadata.annotations.node\.deckhouse\.io\/initial-host-ip`).String()).To(Equal("1.2.3.4"))
		})

		Context("Changing host ip", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-test
  namespace: d8-system
  labels:
    app: deckhouse
  annotations:
    node.deckhouse.io/initial-host-ip: "1.2.3.4"
status:
  hostIP: 4.5.6.7
`, 2))
				f.RunHook()
			})

			It("Should delete the pod", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("Pod", "d8-system", "deckhouse-test").Exists()).To(BeFalse())
			})
		})
	})

	Context("With same initial ip and host ip", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-test
  namespace: d8-system
  labels:
    app: deckhouse
  annotations:
    node.deckhouse.io/initial-host-ip: "1.2.3.4"
status:
  hostIP: 1.2.3.4
`, 1))
			f.RunHook()
		})

		It("Should leave the pod as it is", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-system", "deckhouse-test")
			Expect(pod.Exists()).To(BeTrue())
			Expect(pod.Field(`metadata.annotations.node\.deckhouse\.io\/initial-host-ip`).String()).To(Equal("1.2.3.4"))
		})
	})

	Context("With empty host ip", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-test
  namespace: d8-system
  labels:
    app: deckhouse
  annotations:
    node.deckhouse.io/initial-host-ip: "1.2.3.4"
status: {}
`, 1))
			f.RunHook()
		})

		It("Should leave the pod as it is", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-system", "deckhouse-test")
			Expect(pod.Exists()).To(BeTrue())
			Expect(pod.Field(`metadata.annotations.node\.deckhouse\.io\/initial-host-ip`).String()).To(Equal("1.2.3.4"))
		})
	})
})
