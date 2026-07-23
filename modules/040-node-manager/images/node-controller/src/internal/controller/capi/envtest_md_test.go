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

package capi

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
)

// This spec guards the two regressions the cluster benchmarks caught:
//   - the MachineDeployment must be rendered in the FIRST reconcile after NodeGroup
//     creation (the engine is derived, not awaited from status.engine — the status
//     controller is not part of this suite on purpose);
//   - the event filters (For-predicates, MD create filter) must not starve the
//     controller of the creation event.
var _ = Describe("CAPI MachineDeployment rendering", func() {
	const icName = "ubuntu"

	newInstanceClass := func() *unstructured.Unstructured {
		ic := &unstructured.Unstructured{}
		ic.SetAPIVersion("deckhouse.io/v1alpha1")
		ic.SetKind("DVPInstanceClass")
		ic.SetName(icName)
		Expect(unstructured.SetNestedField(ic.Object, "test", "spec", "vmClassName")).To(Succeed())
		return ic
	}

	newNodeGroup := func(name string) *deckhousev1.NodeGroup {
		ng := &deckhousev1.NodeGroup{}
		ng.Name = name
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: icName},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{"zone-a"},
		}
		return ng
	}

	It("renders the MachineDeployment and infrastructure template for a fresh NodeGroup", func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, newInstanceClass()))).To(Succeed())

		ng := newNodeGroup("cap-e2e")
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(suiteCtx, ng)
			Eventually(func() bool {
				mdList := &capiv1beta2.MachineDeploymentList{}
				_ = k8sClient.List(suiteCtx, mdList, client.InNamespace(common.MachineNamespace),
					client.MatchingLabels{"node-group": ng.Name})
				return len(mdList.Items) == 0
			}, 30*time.Second, 250*time.Millisecond).Should(BeTrue(), "MDs must be cleaned up on NodeGroup deletion")
		})

		var md *capiv1beta2.MachineDeployment
		Eventually(func() error {
			mdList := &capiv1beta2.MachineDeploymentList{}
			if err := k8sClient.List(suiteCtx, mdList, client.InNamespace(common.MachineNamespace),
				client.MatchingLabels{"node-group": ng.Name}); err != nil {
				return err
			}
			if len(mdList.Items) != 1 {
				return fmt.Errorf("expected 1 MachineDeployment, got %d", len(mdList.Items))
			}
			md = &mdList.Items[0]
			return nil
		}, 20*time.Second, 250*time.Millisecond).Should(Succeed())

		By("the MD references the rendered infrastructure template and a stable bootstrap secret name")
		infraRef := md.Spec.Template.Spec.InfrastructureRef
		Expect(infraRef.Kind).To(Equal("DeckhouseMachineTemplate"))
		Expect(md.Spec.Template.Spec.Bootstrap.DataSecretName).NotTo(BeNil())
		Expect(*md.Spec.Template.Spec.Bootstrap.DataSecretName).To(HavePrefix(ng.Name + "-"))

		By("the infrastructure template exists and is owned by node-controller's SSA apply")
		mt := &unstructured.Unstructured{}
		mt.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1alpha1")
		mt.SetKind("DeckhouseMachineTemplate")
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: common.MachineNamespace, Name: infraRef.Name,
		}, mt)).To(Succeed())
		Expect(mt.GetLabels()).To(HaveKeyWithValue("node-group", ng.Name))
	})
})
