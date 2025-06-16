/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: set_cri ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	const (
		stateEmpty = ``

		stateNG1 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-1
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    annotations:
      test-annot: test-annot
    labels:
      test-label: "test-label"
  nodeType: CloudStatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: Auto
`

		stateNG2 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-2
spec:
  cri:
    type: ContainerdV2
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    annotations:
      test-annot: test-annot
    labels:
      test-label: "test-label2"
  nodeType: CloudStatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: Auto
`
	)

	Context("NodeGroup without CRI", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNG1, 1))
			f.RunHook()
		})

		It("should set spec.cri.type to Containerd", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng-1").Field("spec.cri.type").String() == "Containerd")
		})
	})

	Context("NodeGroup with existing CRI", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNG2, 1))
			f.RunHook()
		})

		It("should preserve the existing spec.cri.type", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng-1").Field("spec.cri.type").String() == "ContainerdV2")
		})
	})
})
