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

package fencing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const nodeGroupsYAML = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: legacy
spec:
  nodeType: Static
  fencing:
    mode: Watchdog
    watchdog:
      timeout: 60s
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: notify
spec:
  nodeType: Static
  fencing:
    mode: Notify
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: old-off
spec:
  nodeType: Static
  fencing:
    mode: "off"
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: critical
spec:
  nodeType: Static
  fencing:
    mode: Critical
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: normal
spec:
  nodeType: Static
  fencing:
    mode: Normal
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: slow
spec:
  nodeType: Static
  fencing:
    mode: Slow
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: explicit-disabled
spec:
  nodeType: Static
  fencing:
    mode: Disabled
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: no-fencing
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: engine
spec:
  nodeType: Static
  systemType: Immutable
  fencing:
    mode: Normal
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: explicit-mutable
spec:
  nodeType: Static
  systemType: Mutable
`

func registerNodeGroupCRD(f *HookExecutionConfig) {
	nodeGroupResource := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "NodeGroup", false)
}

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: node_group_modes ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"fencing":{}}}}`, `{}`)
	registerNodeGroupCRD(f)

	Context("Cluster without NodeGroups", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunGoHook()
		})

		It("Must write an empty map", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.fencing.nodeGroups").Map()).To(BeEmpty())
		})
	})

	Context("NodeGroups with current and old modes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupsYAML))
			f.RunGoHook()
		})

		It("Must write the normalized mode of every group", func() {
			Expect(f).To(ExecuteSuccessfully())

			modes := map[string]string{}
			for name, ng := range f.ValuesGet("nodeManager.internal.fencing.nodeGroups").Map() {
				modes[name] = ng.Get("mode").String()
			}
			Expect(modes).To(Equal(map[string]string{
				"legacy":            "Slow",
				"notify":            "Disabled",
				"old-off":           "Disabled",
				"critical":          "Critical",
				"normal":            "Normal",
				"slow":              "Slow",
				"explicit-disabled": "Disabled",
				"no-fencing":        "Disabled",
				"engine":            "Normal",
				"explicit-mutable":  "Disabled",
			}))
		})

		It("Must set the Engine flag for systemType Immutable only", func() {
			Expect(f).To(ExecuteSuccessfully())

			engines := map[string]bool{}
			for name, ng := range f.ValuesGet("nodeManager.internal.fencing.nodeGroups").Map() {
				engines[name] = ng.Get("engine").Bool()
			}
			Expect(engines).To(HaveLen(10))
			for name, engine := range engines {
				Expect(engine).To(Equal(name == "engine"), name)
			}
		})

		It("Must not change the NodeGroup objects", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeGroup", "legacy").Field("spec.fencing.mode").String()).To(Equal("Watchdog"))
			Expect(f.KubernetesGlobalResource("NodeGroup", "notify").Field("spec.fencing.mode").String()).To(Equal("Notify"))
		})
	})
})

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: node_group_modes :: editions ::", func() {
	const criticalGroupYAML = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: critical
spec:
  nodeType: Static
  fencing:
    mode: Critical
`

	for _, tc := range []struct{ edition, mode string }{
		{edition: "CE", mode: "Normal"},
		{edition: "EE", mode: "Critical"},
	} {
		Context("Edition "+tc.edition, func() {
			f := HookExecutionConfigInit(`{"global":{"deckhouseEdition":"`+tc.edition+`"},"nodeManager":{"internal":{"fencing":{}}}}`, `{}`)
			registerNodeGroupCRD(f)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(criticalGroupYAML))
				f.RunGoHook()
			})

			It("Must write the mode "+tc.mode+" for a Critical group and not change the NodeGroup", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.fencing.nodeGroups.critical.mode").String()).To(Equal(tc.mode))
				Expect(f.KubernetesGlobalResource("NodeGroup", "critical").Field("spec.fencing.mode").String()).To(Equal("Critical"))
			})
		})
	}
})
