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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: control_plane_node_roles", func() {

	f := HookExecutionConfigInit(`{"global": {"internal": {"modules": {}}}}`, `{}`)
	Context("Cluster without master nodes (unmanaged)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook just runs", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	assertBothNodeRoles := func(name string) {
		labels := f.KubernetesGlobalResource("Node", name).Parse().Get("metadata.labels")

		value, ok := labels.Map()["node-role.kubernetes.io/master"]
		Expect(ok).To(BeTrue(), "node-role.kubernetes.io/master is not set")
		Expect(value.Str).To(Equal(""), "node-role.kubernetes.io/master value is not empty")

		value, ok = labels.Map()["node-role.kubernetes.io/control-plane"]
		Expect(ok).To(BeTrue(), "node-role.kubernetes.io/control-plane is not set")
		Expect(value.Str).To(Equal(""), "node-role.kubernetes.io/control-plane value is not empty")
	}

	Context("Cluster with master node role, but without control-plane node role", func() {
		names := []string{"kube-master-1", "kube-master-2", "kube-master-3"}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(masterNodeYAMLs(names...)))
			f.RunHook()
		})

		It("Sets control-plane role", func() {
			Expect(f).To(ExecuteSuccessfully())

			for _, name := range names {
				assertBothNodeRoles(name)
			}

		})
	})

	Context("Cluster with control plane node role, but without master node role", func() {
		names := []string{"kube-master-1", "kube-master-2", "kube-master-3"}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(controlPlaneNodeYAMLs(names...)))
			f.RunHook()
		})

		It("Sets master role", func() {
			Expect(f).To(ExecuteSuccessfully())
			for _, name := range names {
				assertBothNodeRoles(name)
			}

		})
	})

	Context("Cluster with both control plane node roles set", func() {
		names := []string{"kube-master-1", "kube-master-2", "kube-master-3"}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(bothRolesNodeYAMLs(names...)))
			f.RunHook()
		})

		It("Preserves both roles", func() {
			Expect(f).To(ExecuteSuccessfully())

			for _, name := range names {
				assertBothNodeRoles(name)
			}

		})
	})
})

func masterNodeYAMLs(names ...string) string {
	return nodeRoleYAMLs([]string{"master"}, names)
}

func controlPlaneNodeYAMLs(names ...string) string {
	return nodeRoleYAMLs([]string{"control-plane"}, names)
}

func bothRolesNodeYAMLs(names ...string) string {
	return nodeRoleYAMLs([]string{"master", "control-plane"}, names)
}

func nodeRoleYAMLs(roles, names []string) string {
	yamls := make([]string, 0, len(names))

	for _, name := range names {
		node := v1.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		labels := make(map[string]string)
		for _, role := range roles {
			labels[fmt.Sprintf("node-role.kubernetes.io/%s", role)] = ""
		}
		node.SetLabels(labels)

		y, err := yaml.Marshal(node)
		if err != nil {
			panic(err)
		}
		yamls = append(yamls, string(y))
	}

	state := strings.Join(yamls, "\n---\n")
	return state
}
