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

var _ = Describe("Modules :: node-manager :: hooks :: migrate_add_status_subresurce_to_node_user_test ::", func() {
	f := HookExecutionConfigInit(`
global: {}
nodeManager:
  internal: {}
`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeUser", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 1))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with and without status in node users", func() {
		nuWithoutStatus := `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: test1
  uid: c4a4e1e2-d56f-4967-b142-b4602c41f4bf
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: $6$YO.
  sshPublicKey: ssh-rsa AAAAB
  uid: 1001
---
`
		nuWithStatus := `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: test2
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: $6$Y.
  sshPublicKey: ssh-rsa AAA
  uid: 1005
status:
  errors: {}
---
`
		BeforeEach(func() {
			f.KubeStateSet(nuWithoutStatus + nuWithStatus)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should set status for node user without status", func() {
			Expect(f).To(ExecuteSuccessfully())

			nu := f.KubernetesResource("NodeUser", "", "test1")

			Expect(nu.Exists()).To(BeTrue())
			Expect(nu.Field("status").Exists()).To(BeTrue())
			Expect(nu.Field("status.errors").Exists()).To(BeTrue())
		})

		It("Hook should not change user with status", func() {
			Expect(f).To(ExecuteSuccessfully())

			nu := f.KubernetesResource("NodeUser", "", "test2")

			Expect(nu.Exists()).To(BeTrue())
			Expect(nu.ToYaml()).To(MatchYAML(nuWithStatus))
		})
	})
})
