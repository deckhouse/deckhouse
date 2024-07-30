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

var _ = Describe("Modules :: node-manager :: hooks :: set_priorities ::", func() {
	const (
		ngsWithPriorities = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3
    priority: 20
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng2
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3
    priority: 50
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng3
spec:
  cloudInstances:
    maxPerZone: 10
    minPerZone: 6
`
		ngsWithoutPriorities = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng2
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3
`
	)
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}, "instancePrefix": "test"}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("With priorities", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ngsWithPriorities))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.ValuesGet("nodeManager.internal.clusterAutoscalerPriorities").String()
			Expect(m).To(Equal(`{"1":[".*"],"20":["^test-ng1-[0-9a-zA-Z]+$"],"50":["^test-ng2-[0-9a-zA-Z]+$"]}`))
		})
	})

	Context("Without priorities", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ngsWithoutPriorities))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.ValuesGet("nodeManager.internal.clusterAutoscalerPriorities").Exists()
			Expect(m).To(BeFalse())
		})
	})

})
