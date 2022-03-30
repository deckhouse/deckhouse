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

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	internal "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Global hooks :: migrate :: add_control_plane_role_to_master_ng_test ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

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
		masterNgWithoutRoleAndIncludeLBLabel = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    labels:
      node-role.kubernetes.io/master: ""
      node.kubernetes.io/exclude-from-external-load-balancers: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudStatic
`
		masterNgWithRoleAndExcludeLBLabel = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    labels:
      node-role.kubernetes.io/master: ""
      node-role.kubernetes.io/control-plane: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudStatic
`
	)

	Context("Cluster without node groups", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook execute successfully and node groups should not created", func() {
			Expect(f).To(ExecuteSuccessfully())

			nodeGroups, err := f.KubeClient().Dynamic().Resource(nodeGroupResource).Namespace("").List(context.TODO(), v1.ListOptions{})

			Expect(err).ToNot(HaveOccurred())
			Expect(nodeGroups.Items).To(HaveLen(0))
		})
	})

	Context("Cluster has master node group without role", func() {
		BeforeEach(func() {
			JoinKubeResourcesAndSet(f, masterNgWithoutRoleAndIncludeLBLabel, workerNgYAML)
			f.RunHook()
		})

		It("Sets role for master node group and not affect another labels", func() {
			Expect(f).To(ExecuteSuccessfully())

			masterNg := f.KubernetesResource("NodeGroup", "", "master")
			labels := masterNg.Field("spec.nodeTemplate.labels").Map()

			Expect(labels).To(HaveKey(controlPlaneRoleLabel))
			Expect(labels).To(HaveKey("node-role.kubernetes.io/master"))

			Expect(labels).ToNot(HaveKey(excludeLoadBalancerLabel))
		})

		It("Should not affect another fields in spec", func() {
			Expect(f).To(ExecuteSuccessfully())

			masterNg := f.KubernetesResource("NodeGroup", "", "master")
			labels := masterNg.Field("spec.nodeTemplate.labels").Map()

			Expect(labels).To(HaveKey("node-role.kubernetes.io/master"))

			taints := masterNg.Field("spec.nodeTemplate.taints").Array()

			Expect(taints).To(HaveLen(1))
			Expect(taints[0].Value()).To(Equal(map[string]interface{}{
				"effect": "NoSchedule",
				"key":    "node-role.kubernetes.io/master",
			}))
		})

		It("Does not affect another ng", func() {
			Expect(f).To(ExecuteSuccessfully())

			workerNg := f.KubernetesResource("NodeGroup", "", "worker")
			Expect(workerNg.ToYaml()).To(MatchYAML(workerNgYAML))
		})
	})

	Context("Cluster has master node group with role", func() {
		BeforeEach(func() {
			JoinKubeResourcesAndSet(f, masterNgWithRoleAndExcludeLBLabel, workerNgYAML)
			f.RunHook()
		})

		It("Should not affect node groups", func() {
			Expect(f).To(ExecuteSuccessfully())

			masterNg := f.KubernetesResource("NodeGroup", "", "master")
			workerNg := f.KubernetesResource("NodeGroup", "", "worker")

			var masterNgExpected internal.NodeGroup
			var masterNgInCluster internal.NodeGroup

			err := yaml.Unmarshal([]byte(masterNgWithRoleAndExcludeLBLabel), &masterNgExpected)
			Expect(err).ToNot(HaveOccurred())

			err = yaml.Unmarshal([]byte(masterNg.ToYaml()), &masterNgInCluster)
			Expect(err).ToNot(HaveOccurred())

			Expect(masterNgExpected).To(Equal(masterNgInCluster))

			Expect(workerNg.ToYaml()).To(MatchYAML(workerNgYAML))
		})
	})
})
