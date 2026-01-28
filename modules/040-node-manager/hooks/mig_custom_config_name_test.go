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
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	customNgYAML = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: custom-ng
spec:
  nodeType: Static
  gpu:
    sharing: MIG
    mig:
      partedConfig: custom
      customConfigs:
        - index: 0
          slices:
            - profile: 1g.10gb
              count: 2
        - index: 1
          slices:
            - profile: 2g.20gb
`
)

var _ = Describe("node-manager :: hooks :: mig_custom_config_name ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "NodeGroup", false)

	Context("NodeGroup with custom MIG config", func() {
		BeforeEach(func() {
			f.KubeStateSet(customNgYAML)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should populate resolved name in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			val := f.ValuesGet("nodeManager.internal.customMIGNames.custom-ng").String()
			Expect(val).To(ContainSubstring("custom-ng-"))
		})
	})

	Context("Order-insensitive hashing", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: shuffled-ng
spec:
  nodeType: Static
  gpu:
    sharing: MIG
    mig:
      partedConfig: custom
      customConfigs:
        - index: 1
          slices:
            - profile: 1g.10gb
              count: 2
            - profile: 2g.20gb
        - index: 0
          slices:
            - profile: 2g.20gb
            - profile: 1g.10gb
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should produce the same name despite order", func() {
			Expect(f).To(ExecuteSuccessfully())
			val := f.ValuesGet("nodeManager.internal.customMIGNames.shuffled-ng").String()
			Expect(val).To(Equal("custom-shuffled-ng-8ab188ce"))
		})
	})
})
