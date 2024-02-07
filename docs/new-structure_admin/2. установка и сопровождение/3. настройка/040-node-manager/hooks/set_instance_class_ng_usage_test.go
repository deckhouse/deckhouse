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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: set_instance_class_usage ::", func() {
	// inject Kind, we don't have dynamic reload in tests
	setInstanceClassNGUsageConfig.Kubernetes[0] = go_hook.KubernetesConfig{
		Name:                "ics",
		ApiVersion:          "deckhouse.io/v1",
		Kind:                "OpenStackInstanceClass",
		ExecuteHookOnEvents: pointer.Bool(false),
		FilterFunc:          applyUsedInstanceClassFilter,
	}

	f := HookExecutionConfigInit(`
global: {}
nodeManager:
  internal: {}
`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "OpenStackInstanceClass", false)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("NodeGroups have InstanceClass linked", func() {
		BeforeEach(func() {
			state := `
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  flavorName: m1.large
  imageName: ubuntu-22-04-cloud-amd64
  mainNetwork: test
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker
    maxPerZone: 3
    minPerZone: 3
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: another-worker
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker
    maxPerZone: 0
    minPerZone: 2
  nodeType: CloudEphemeral
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, 1))
			f.RunHook()
		})

		It("NodeGroupConsumers should be not empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			worker := f.KubernetesGlobalResource("OpenStackInstanceClass", "worker")
			Expect(worker.Field("status.nodeGroupConsumers").Array()).To(HaveLen(2))
			Expect(worker.Field("status.nodeGroupConsumers").Array()).To(ContainElements(ContainSubstring("worker"), ContainSubstring("another-worker")))
		})
	})

	Context("NodeGroup has another InstanceClass", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  flavorName: m1.large
  imageName: ubuntu-22-04-cloud-amd64
  mainNetwork: test
status:
  nodeGroupConsumers:
    - old-worker
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: next
    maxPerZone: 3
    minPerZone: 3
  nodeType: CloudEphemeral
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, 1))
			f.RunHook()
		})

		It("NodeGroupConsumers should be absent", func() {
			Expect(f).To(ExecuteSuccessfully())
			worker := f.KubernetesGlobalResource("OpenStackInstanceClass", "worker")
			Expect(worker.Field("status.nodeGroupConsumers").Array()).To(HaveLen(0))
		})
	})

	Context("NodeGroup InstanceClass was changed", func() {
		BeforeEach(func() {
			state := `
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  flavorName: m1.large
  imageName: ubuntu-22-04-cloud-amd64
  mainNetwork: test
status:
  nodeGroupConsumers:
    - old-worker
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker
    maxPerZone: 3
    minPerZone: 3
  nodeType: CloudEphemeral
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, 1))
			f.RunHook()
		})

		It("NodeGroupConsumers should change the value", func() {
			Expect(f).To(ExecuteSuccessfully())
			worker := f.KubernetesGlobalResource("OpenStackInstanceClass", "worker")
			Expect(worker.Field("status.nodeGroupConsumers").Array()).To(HaveLen(1))
			Expect(worker.Field("status.nodeGroupConsumers").Array()).To(ContainElements(ContainSubstring("worker")))
			Expect(worker.Field("status.nodeGroupConsumers").Array()).ToNot(ContainElements(ContainSubstring("old-worker")))
		})
	})
})
