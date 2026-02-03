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
1. Hook must discover number of control-plane Nodes and save to global.discovery.clusterMasterCount.
2. Hook must determine HA mode based on desired master replicas from the master NodeGroup.
3. If NodeGroup desired replicas are unknown, use clusterConfiguration.masterNodeGroup.replicas.
4. If desired replicas are unknown, fallback to current master count.

*/

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Global hooks :: discovery :: cluster_ha ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
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

		stateMasterNodeGroupCloud = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  cloudInstances:
    minPerZone: 1
    zones:
    - zone-a
    - zone-b
`

		stateMasterNodeGroupCloudNoZones = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  cloudInstances:
    minPerZone: 2
`

		stateMasterNodeGroupStatic = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  staticInstances:
    count: 1
`

		clusterConfigurationStatic = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.30"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
clusterDomain: cluster.local
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	nodeGroupGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

	applyNodeGroup := func(cfg *HookExecutionConfig, state string) {
		obj := &unstructured.Unstructured{}
		payload, err := yaml.YAMLToJSON([]byte(state))
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.UnmarshalJSON(payload)).To(Succeed())

		_, err = cfg.KubeClient().Dynamic().Resource(nodeGroupGVR).Create(context.TODO(), obj, v1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	Context("NodeGroup CRD is missing", func() {
		fNoNodeGroupCRD := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			fNoNodeGroupCRD.ValuesDelete("global.clusterConfiguration")
			fNoNodeGroupCRD.BindingContexts.Set(fNoNodeGroupCRD.KubeStateSet(stateFirstMasterNode))
			fNoNodeGroupCRD.RunHook()
		})

		It("Must be executed successfully without NodeGroup CRD", func() {
			Expect(fNoNodeGroupCRD).To(ExecuteSuccessfully())
			Expect(fNoNodeGroupCRD.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(fNoNodeGroupCRD.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.ValuesDelete("global.clusterConfiguration")
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
			f.ValuesDelete("global.clusterConfiguration")
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
				f.ValuesDelete("global.clusterConfiguration")
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

	Context("Master NodeGroup defines desired replicas", func() {
		Context("Cloud node group with zones", func() {
			BeforeEach(func() {
				f.ValuesDelete("global.clusterConfiguration")
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode))
				applyNodeGroup(f, stateMasterNodeGroupCloud)
				f.RunHook()
			})

			It("must set HA based on minPerZone * zones", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})

		Context("Cloud node group without zones", func() {
			BeforeEach(func() {
				f.ValuesDelete("global.clusterConfiguration")
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode))
				applyNodeGroup(f, stateMasterNodeGroupCloudNoZones)
				f.RunHook()
			})

			It("must treat minPerZone as desired replicas", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})

		Context("Static node group with count 1 and two masters", func() {
			BeforeEach(func() {
				f.ValuesDelete("global.clusterConfiguration")
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				applyNodeGroup(f, stateMasterNodeGroupStatic)
				f.RunHook()
			})

			It("must not enable HA when desired replicas is 1", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
			})
		})

		Context("NodeGroup takes precedence over clusterConfiguration", func() {
			BeforeEach(func() {
				f.ValuesDelete("global.clusterConfiguration")
				f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationStatic))
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				applyNodeGroup(f, stateMasterNodeGroupStatic)
				f.RunHook()
			})

			It("must follow NodeGroup desired replicas even when clusterConfiguration conflicts", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
			})
		})
	})

	Context("ClusterConfiguration defines desired replicas", func() {
		BeforeEach(func() {
			f.ValuesDelete("global.clusterConfiguration")
		})

		Context("clusterConfiguration present with two masters", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationStatic))
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				f.RunHook()
			})

			It("must fallback to current masters count", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})

		Context("clusterConfiguration present with one master", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationStatic))
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode))
				f.RunHook()
			})

			It("must fallback to current masters count", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())
			})
		})

		Context("clusterConfiguration exists without masterNodeGroup.replicas", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationStatic))
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				f.RunHook()
			})

			It("must fallback to current masters count", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})
	})
})
