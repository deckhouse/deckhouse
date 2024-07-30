/*
Copyright 2021 Flant JSC

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
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-ping:: hooks :: discover_targets ::", func() {
	const (
		node1 = `
---
apiVersion: v1
kind: Node
metadata:
  name: system
  labels:
    node-role.deckhouse.io/system: ""
status:
  addresses:
  - address: 192.168.199.213
    type: InternalIP
  - address: 95.217.82.168
    type: ExternalIP
  - address: master
    type: Hostname
`
		node2 = `
---
apiVersion: v1
kind: Node
metadata:
  name: system2
  labels:
    node-role.deckhouse.io/system: ""
status:
  addresses:
  - address: 192.168.199.140
    type: InternalIP
  - address: worker
    type: Hostname
`
		unschedulableNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: unschedulable-system
  labels:
    node-role.deckhouse.io/system: ""
status:
  addresses:
  - address: 192.168.199.140
    type: InternalIP
  - address: worker
    type: Hostname
spec:
  unschedulable: true
`
		nodeWithEmptyIP = `
---
apiVersion: v1
kind: Node
metadata:
  name: system3
  labels:
    node-role.deckhouse.io/system: ""
status:
  addresses:
  - address: ""
    type: InternalIP
  - address: worker
    type: Hostname
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringPing":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("One node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1))
			f.RunHook()
		})
		It("Hook must not fail, monitoringPing.internal.targets must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringPing.internal.targets").String()).To(MatchUnorderedJSON(`
{
          "cluster_targets": [
            {
              "ipAddress": "192.168.199.213",
              "name": "system"
            }
          ],
          "external_targets": []
        }
`))
		})
	})

	Context("Two nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1 + node2))
			f.RunHook()
		})
		It("Hook must not fail, monitoringPing.internal.targets must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringPing.internal.targets").String()).To(MatchUnorderedJSON(`
 {
          "cluster_targets": [
            {
              "ipAddress": "192.168.199.140",
              "name": "system2"
            },
            {
              "ipAddress": "192.168.199.213",
              "name": "system"
            }
          ],
          "external_targets": []
        }
`))
		})
	})

	Context("Two nodes with external targets", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1 + node2))
			f.ValuesSetFromYaml("monitoringPing.externalTargets", []byte(`[
{ "host": "1.2.3.4" },
{ "host": "5.6.7.8" }
]`))
			f.RunHook()
		})
		It("Hook must not fail, monitoringPing.internal.targets must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringPing.internal.targets").String()).To(MatchUnorderedJSON(`
 {
          "cluster_targets": [
            {
              "ipAddress": "192.168.199.140",
              "name": "system2"
            },
            {
              "ipAddress": "192.168.199.213",
              "name": "system"
            }
          ],
          "external_targets": [
            {
              "host": "1.2.3.4"
            },
            {
              "host": "5.6.7.8"
            }
          ]
        }
`))
		})
	})

	Context("One node ready, one unschedulable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1 + unschedulableNode))
			f.RunHook()
		})
		It("Hook must not fail, monitoringPing.internal.targets must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringPing.internal.targets").String()).To(MatchUnorderedJSON(`
 {
          "cluster_targets": [
            {
              "ipAddress": "192.168.199.213",
              "name": "system"
            }
          ],
          "external_targets": []
        }
`))
		})
	})

	Context("One node ready, one without IP", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1 + nodeWithEmptyIP))
			f.RunHook()
		})
		It("Hook must not fail, monitoringPing.internal.targets must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringPing.internal.targets").String()).To(MatchUnorderedJSON(`
 {
          "cluster_targets": [
            {
              "ipAddress": "192.168.199.213",
              "name": "system"
            }
          ],
          "external_targets": []
        }
`))
		})
	})
})
