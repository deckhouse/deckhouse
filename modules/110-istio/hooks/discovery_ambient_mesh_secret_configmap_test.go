/*
Copyright 2025 Flant JSC

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

var _ = Describe("Istio hooks :: discovery_ambient_mesh_secret_configmap ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should set ambient mode to false when no configmap exists", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeFalse())
		})
	})

	Context("Cluster with ambient mode configmap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-istio
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-ambientmode
  namespace: d8-istio
`))
			f.RunHook()
		})

		It("Should set ambient mode to true when configmap exists", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeTrue())
		})
	})

	Context("Cluster with configmap in wrong namespace", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: wrong-namespace
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-ambientmode
  namespace: wrong-namespace
`))
			f.RunHook()
		})

		It("Should set ambient mode to false when configmap is in wrong namespace", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeFalse())
		})
	})
})
