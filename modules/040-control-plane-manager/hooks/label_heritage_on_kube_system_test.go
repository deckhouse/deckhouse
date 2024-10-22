/*
Copyright 2024 Flant JSC

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

var _ = Describe("Modules :: control-plane-manager :: hooks :: label_heritage_on_kube_system ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse": { "internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has ns kube-system with label heritage", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: kube-system
  name: kube-system
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Namespace", "kube-system").ToYaml()).To(MatchYAML(`
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: kube-system
  name: kube-system
`))
		})
	})
	Context("Cluster has ns kube-system", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    extended-monitoring.deckhouse.io/enabled: ""
    kubernetes.io/metadata.name: kube-system
  name: kube-system
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Namespace", "kube-system").ToYaml()).To(MatchYAML(`
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: kube-system
  name: kube-system
`))
		})
	})
})
