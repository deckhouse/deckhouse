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

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: migrate_system_reserve ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("NodeGroup is deleted after the snapshot is taken", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: deleted
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: remaining
spec:
  nodeType: Static
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			nodeGroups := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
			err := f.KubeClient().Dynamic().Resource(nodeGroups).Delete(context.Background(), "deleted", metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			f.RunHook()
		})

		It("Hook must not fail and must migrate the remaining NodeGroup", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeGroup", "deleted").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("NodeGroup", "remaining").Field("spec.kubelet.resourceReservation.mode").String()).To(Equal("Off"))
			Expect(f.KubernetesResource("ConfigMap", "d8-system", "kubelet-resource-reservation-migration").Exists()).To(BeTrue())
		})
	})
})
