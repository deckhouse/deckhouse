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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: reconcile-masters-node ::", func() {
	var (
		initValuesString = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
	)
	const (
		initConfigValuesString = ``
	)

	var (
		reconcileStartState = `
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.1
      type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.2
      type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-2
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.3
      type: InternalIP
`

		reconcileChangedState = strings.Join(strings.Split(reconcileStartState, "---")[:3], "---")
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Multimaster cluster set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(reconcileStartState))
			f.RunHook()

			Expect(f.ValuesGet("controlPlaneManager.internal.mastersNode").Exists()).To(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.internal.mastersNode").String()).To(MatchJSON(`[ "main-master-0", "main-master-1", "main-master-2" ]`))
		})

		It("Hook is running successfully", func() {
			Expect(f).Should(ExecuteSuccessfully())
		})

		Context("main-master-2 was removed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(reconcileChangedState))
				f.RunHook()
			})

			It("Expects main-master-2 etcd member was removed", func() {
				Expect(f).Should(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.mastersNode").Exists()).To(BeTrue())
				Expect(f.ValuesGet("controlPlaneManager.internal.mastersNode").String()).To(MatchJSON(`[ "main-master-0", "main-master-1" ]`))
			})
		})
	})
})
