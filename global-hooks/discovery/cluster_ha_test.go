// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. Hook must discover number of control-plane Nodes and save to global.discovery.clusterMasterCount,
2. If number of control-plane Nodes is more than one — hook must set global.discovery.clusterControlPlaneIsHighlyAvailable to true, else — to false.
3. If preserveExistingHAMode is enabled and HA value is already set, hook must keep it unchanged.

*/

package hooks

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cluster_ha ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	var (
		stateFirstMasterNode = `
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""`

		stateSecondMasterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""`

		statePreserveExistingHAMode = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-dhctl-converge-state
  namespace: d8-system
data:
  state.json: ` + base64.StdEncoding.EncodeToString([]byte(`{"preserveExistingHAMode":true}`))

		stateInvalidPreserveExistingHAMode = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-dhctl-converge-state
  namespace: d8-system
data:
  state.json: ` + base64.StdEncoding.EncodeToString([]byte(`{"preserveExistingHAMode":`))
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` must be 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("0"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})
	})

	Context("One master node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode))
			f.RunHook()
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` must be 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})

		Context("Two master nodes in cluster", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				f.RunHook()
			})

			It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be true; `global.discovery.clusterMasterCount` must be 2", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})
	})

	Context("Two master nodes with preserve existing HA enabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode + statePreserveExistingHAMode))
			f.RunHook()
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be true; `global.discovery.clusterMasterCount` must be 2", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})
	})

	Context("Preserve existing HA value when it is already set", func() {
		const initValuesWithHA = `{"global": {"discovery": {"clusterControlPlaneIsHighlyAvailable": true}}}`
		fPreserve := HookExecutionConfigInit(initValuesWithHA, initConfigValuesString)

		BeforeEach(func() {
			fPreserve.BindingContexts.Set(fPreserve.KubeStateSet(stateFirstMasterNode + statePreserveExistingHAMode))
			fPreserve.RunHook()
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must stay true; `global.discovery.clusterMasterCount` must be 1", func() {
			Expect(fPreserve).To(ExecuteSuccessfully())
			Expect(fPreserve.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(fPreserve.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(fPreserve.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})
	})

	Context("Invalid converge state secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateInvalidPreserveExistingHAMode))
			f.RunHook()
		})

		It("Must ignore invalid state.json and recalculate HA from master nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
		})
	})
})
