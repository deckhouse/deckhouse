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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type Target struct {
	Name      string `json:"name"`
	IPAddress string `json:"ipAddress"`
}

type Targets struct {
	Cluster []Target `json:"clusterTargets"`
}

var ctxNode1 = `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
spec: {}
status:
  addresses:
    - type: InternalIP
      address: 10.0.0.1
`

var ctxNode2 = `
---
apiVersion: v1
kind: Node
metadata:
  name: node2
spec: {}
status:
  addresses:
    - type: InternalIP
      address: 10.0.0.2
`

var ctxNode3NoAddress = `
---
apiVersion: v1
kind: Node
metadata:
  name: node3
spec: {}
status:
  addresses:
    - type: InternalIP
      address: ""
`

var ctxNode4Unschedulable = `
---
apiVersion: v1
kind: Node
metadata:
  name: node4
spec:
  unschedulable: true
status:
  addresses:
    - type: InternalIP
      address: 10.0.0.4
`

var ctxConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: monitoring-ping-config
  namespace: d8-monitoring
data:
  targets.json: ""
`

var ctxNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-monitoring
`

var _ = Describe("Modules :: monitoring-ping :: hooks :: discover_targets", func() {
	f := HookExecutionConfigInit(
		`{"monitoringPing":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("List nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ctxNode1 + ctxNode2 + ctxNode3NoAddress + ctxNode4Unschedulable + ctxNamespace + ctxConfigMap))
			f.RunGoHook()
		})

		It("Targets should exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			str := f.ValuesGet("monitoringPing.internal.clusterTargets").String()
			var targets []Target
			err := json.Unmarshal([]byte(str), &targets)
			if err != nil {
				panic(err)
			}
			Expect(len(targets)).To(Equal(2))
		})
	})
})
