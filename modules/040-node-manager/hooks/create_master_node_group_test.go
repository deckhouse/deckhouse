// Copyright 2022 Flant JSC
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

package hooks

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: migrate :: add_control_plane_role_to_master_ng_test ::", func() {
	const initValues = `
global:
  clusterConfiguration:
    apiVersion: deckhouse.io/v1alpha1
    cloud:
      prefix: sandbox
      provider: OpenStack
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.29"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
`
	f := HookExecutionConfigInit(initValues, `{}`)

	var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "NodeGroup", false)

	const (
		workerNgYAML = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-big
    maxPerZone: 1
    maxSurgePerZone: 1
    maxUnavailablePerZone: 0
    minPerZone: 1
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node.deckhouse.io/group: worker-big
  nodeType: CloudEphemeral
`
		masterNgNoneDefault = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    annotations:
      test-annot: test-annot
    labels:
      test-label: "test-label"
      node-role.kubernetes.io/master: ""
      node-role.kubernetes.io/control-plane: ""
  nodeType: CloudStatic
`
	)

	assertCountNodeGroups := func(f *HookExecutionConfig, count int) {
		nodeGroups, err := f.KubeClient().Dynamic().Resource(nodeGroupResource).Namespace("").List(context.TODO(), v1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeGroups.Items).To(HaveLen(count))
	}

	for _, clType := range []string{"Cloud", "Static"} {
		clusterType := clType
		Context(fmt.Sprintf("%s cluster", clType), func() {
			masterNgUnstructured, err := getDefaultMasterNg(clusterType)
			if err != nil {
				panic(err)
			}
			var masterNgDefaultYAMLBBytes []byte
			masterNgDefaultYAMLBBytes, err = yaml.Marshal(masterNgUnstructured)
			if err != nil {
				panic(err)
			}

			var masterNgDefaultYAML = string(masterNgDefaultYAMLBBytes)

			assertDefaultMasterNodeGroupOnlyPresent := func(f *HookExecutionConfig) {
				masterNg := f.KubernetesResource("NodeGroup", "", "master")
				Expect(masterNg.ToYaml()).To(MatchYAML(masterNgDefaultYAML))

				assertCountNodeGroups(f, 1)
			}

			BeforeEach(func() {
				f.ValuesSet("global.clusterConfiguration.clusterType", clusterType)
			})

			Context("Cluster without node groups", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})

				It("Should create default master node group only", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertDefaultMasterNodeGroupOnlyPresent(f)
				})
			})

			Context("Cluster has default master node group", func() {
				Context("only", func() {
					BeforeEach(func() {
						JoinKubeResourcesAndSet(f, masterNgDefaultYAML)

						f.RunHook()
					})

					It("should not change master node group", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertDefaultMasterNodeGroupOnlyPresent(f)
					})
				})

				Context("with another ng", func() {
					BeforeEach(func() {
						JoinKubeResourcesAndSet(f, masterNgDefaultYAML, workerNgYAML)

						f.RunHook()
					})

					It("should not change master node group", func() {
						Expect(f).To(ExecuteSuccessfully())

						masterNg := f.KubernetesResource("NodeGroup", "", "master")
						Expect(masterNg.ToYaml()).To(MatchYAML(masterNgDefaultYAML))
					})

					It("should not change another node group", func() {
						Expect(f).To(ExecuteSuccessfully())

						workerNg := f.KubernetesResource("NodeGroup", "", "worker")
						Expect(workerNg.ToYaml()).To(MatchYAML(workerNgYAML))
					})

					It("should not create another node groups", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertCountNodeGroups(f, 2)
					})
				})
			})

			Context("Cluster has none default master node group only", func() {
				Context("only", func() {
					BeforeEach(func() {
						JoinKubeResourcesAndSet(f, masterNgNoneDefault)

						f.RunHook()
					})

					It("Should not change master node group", func() {
						Expect(f).To(ExecuteSuccessfully())

						masterNg := f.KubernetesResource("NodeGroup", "", "master")
						Expect(masterNg.ToYaml()).To(MatchYAML(masterNgNoneDefault))
					})

					It("Should not create another node groups", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertCountNodeGroups(f, 1)
					})
				})

				Context("with another ng", func() {
					BeforeEach(func() {
						JoinKubeResourcesAndSet(f, masterNgNoneDefault, workerNgYAML)

						f.RunHook()
					})

					It("should not change master node group", func() {
						Expect(f).To(ExecuteSuccessfully())

						masterNg := f.KubernetesResource("NodeGroup", "", "master")
						Expect(masterNg.ToYaml()).To(MatchYAML(masterNgNoneDefault))
					})

					It("should not change another node group", func() {
						Expect(f).To(ExecuteSuccessfully())

						workerNg := f.KubernetesResource("NodeGroup", "", "worker")
						Expect(workerNg.ToYaml()).To(MatchYAML(workerNgYAML))
					})

					It("should not create another node groups", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertCountNodeGroups(f, 2)
					})
				})
			})
		})
	}
})
