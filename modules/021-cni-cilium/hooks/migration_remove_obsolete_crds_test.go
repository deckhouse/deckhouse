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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("modules :: cni-cilium :: hooks :: migration-remove-obsolete-crds ::", func() {
	f := HookExecutionConfigInit(`{}`, "")

	Context("Empty cluster.", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Obsolete CRDs are in cluster.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ciliumegressnatpolicies.cilium.io
spec: {}
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ciliumbgploadbalancerippools.cilium.io
spec: {}
`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})
		It("CRDs must disappear", func() {
			fmt.Println(f.GoHookError)
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "ciliumegressnatpolicies.cilium.io").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "ciliumbgploadbalancerippools.cilium.io").Exists()).To(BeFalse())
		})
	})
})
