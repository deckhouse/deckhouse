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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: node-manager :: hooks :: set_priorities ::", func() {
	const (
		staticNGs = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-static-1
spec:
  nodeType: Static
`
		stateNGs = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  cloudInstances:
    maxPerZone: 2
    minPerZone: 5 # $ng_min_instances -ge $ng_max_instances
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng20
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3 # "$replicas" == "null"
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng21
spec:
  priority: 20
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3 # $replicas -eq 0
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng3
spec:
  cloudInstances:
    maxPerZone: 10
    minPerZone: 6 # $replicas -le $ng_min_instances
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng4
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3 # $replicas -gt $ng_max_instances
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng5
spec:
  cloudInstances:
    maxPerZone: 10
    minPerZone: 1 # $ng_min_instances <= $replicas <= $ng_max_instances
`
	)
	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {"instancePrefix": "test"}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGs))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			fmt.Println("AAAAA")
			fmt.Println(f.ValuesGet("nodeManager.internal.clusterAutoscalerPriorities"))
		})
	})

})
