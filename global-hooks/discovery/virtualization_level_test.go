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
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: virtualization_level", func() {

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	Context("Cluster without master nodes (unmanaged)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook just runs", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with 1 hardware master node and 2 virtual ones", func() {
		nodesLevels := map[string]int{
			"kube-master-1": 0,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(masterNodeYAMLs(nodesLevels)))
			f.RunHook()
		})

		It("Create secret, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))

			encoded := base64.StdEncoding.EncodeToString([]byte("0"))

			secret := f.KubernetesResource("Secret", "d8-system", "d8-virtualization-level")
			Expect(secret.Field("data.level").String()).To(Equal(encoded))
		})
	})

	Context("Cluster with 3 virtual master nodes", func() {
		nodesLevels := map[string]int{
			"kube-master-1": 1,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(masterNodeYAMLs(nodesLevels)))
			f.RunHook()
		})

		It("Create secret, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("1"))

			encoded := base64.StdEncoding.EncodeToString([]byte("1"))

			secret := f.KubernetesResource("Secret", "d8-system", "d8-virtualization-level")
			Expect(secret.Field("data.level").String()).To(Equal(encoded))
		})
	})
})

func masterNodeYAMLs(nodeLevels map[string]int) string {
	return nodeRoleYAMLs([]string{"master"}, nodeLevels)
}

func nodeRoleYAMLs(roles []string, nodeLevels map[string]int) string {
	yamls := make([]string, 0, len(nodeLevels))

	for name, level := range nodeLevels {
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
		labels[masterNodeGroup] = "master"
		labels[virtualizationLevelKey] = fmt.Sprintf("%d", level)
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
