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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
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
		Expect(unstructured.SetNestedMap(ic.Object, map[string]interface{}{
			"cpu": map[string]interface{}{
				"cores":        int64(4),
				"coreFraction": "100%",
			},
			"memory": map[string]interface{}{"size": "8Gi"},
		}, "spec", "virtualMachine")).To(Succeed())
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

	It("publishes CPU and memory capacity for scale-from-zero", func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, newInstanceClass()))).To(Succeed())

		ng := newNodeGroup(testenv.UniqueName("cap-zero"))
		ng.Spec.CloudInstances.MinPerZone = 0
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ng) })

		Eventually(func(g Gomega) map[string]string {
			mdList := &capiv1beta2.MachineDeploymentList{}
			g.Expect(k8sClient.List(suiteCtx, mdList, client.InNamespace(common.MachineNamespace),
				client.MatchingLabels{"node-group": ng.Name})).To(Succeed())
			g.Expect(mdList.Items).To(HaveLen(1))
			return mdList.Items[0].Annotations
		}, 20*time.Second, 250*time.Millisecond).Should(SatisfyAll(
			HaveKeyWithValue("capacity.cluster-autoscaler.kubernetes.io/cpu", "4"),
			HaveKeyWithValue("capacity.cluster-autoscaler.kubernetes.io/memory", "8Gi"),
		))
	})

	// Before the migration helm rendered the infrastructure template and pruned it when the
	// NodeGroup left its values. node-controller took the rendering over, and for a while
	// nothing took the deletion over: a live cluster ended up with one orphaned template per
	// zone for every NodeGroup ever deleted.
	It("deletes the infrastructure template when the NodeGroup is removed", func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, newInstanceClass()))).To(Succeed())

		ng := newNodeGroup(testenv.UniqueName("cap-teardown"))
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())

		templates := func(g Gomega) []unstructured.Unstructured {
			list := &unstructured.UnstructuredList{}
			list.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1alpha1")
			list.SetKind("DeckhouseMachineTemplateList")
			g.Expect(k8sClient.List(suiteCtx, list, client.InNamespace(common.MachineNamespace),
				client.MatchingLabels{"node-group": ng.Name})).To(Succeed())
			return list.Items
		}

		Eventually(func(g Gomega) int { return len(templates(g)) },
			20*time.Second, 250*time.Millisecond).Should(Equal(1), "the template must be rendered first")

		Expect(k8sClient.Delete(suiteCtx, ng)).To(Succeed())

		Eventually(func(g Gomega) int { return len(templates(g)) },
			30*time.Second, 250*time.Millisecond).Should(BeZero(), "no template may outlive its NodeGroup")

		Eventually(func() bool {
			got := &deckhousev1.NodeGroup{}
			return apierrors.IsNotFound(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(ng), got))
		}, 30*time.Second, 250*time.Millisecond).Should(BeTrue(), "the finalizer must be released")
	})

	// Rolling nodes is never acceptable. dataSecretName is part of spec.template, so rewriting
	// it replaces the MachineSet and recreates every node of the group. MachineDeployments
	// created before the Secret name became zone-based carry a checksum-based name, and the
	// reconciler must keep it instead of applying the name it would compute today.
	It("never rewrites the bootstrap secret name of an existing MachineDeployment", func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, newInstanceClass()))).To(Succeed())

		ng := newNodeGroup("cap-adopt")
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ng) })

		mdKey := types.NamespacedName{}
		Eventually(func() error {
			mdList := &capiv1beta2.MachineDeploymentList{}
			if err := k8sClient.List(suiteCtx, mdList, client.InNamespace(common.MachineNamespace),
				client.MatchingLabels{"node-group": ng.Name}); err != nil {
				return err
			}
			if len(mdList.Items) != 1 {
				return fmt.Errorf("expected 1 MachineDeployment, got %d", len(mdList.Items))
			}
			mdKey = client.ObjectKeyFromObject(&mdList.Items[0])
			return nil
		}, 20*time.Second, 250*time.Millisecond).Should(Succeed())

		By("pinning a legacy, checksum-style bootstrap secret name on the existing MD")
		const legacyName = "cap-adopt-legacychecksum"
		// The reconcile that created this MachineDeployment may still be in flight, holding the
		// computed name it read before the pin; its apply would overwrite the pin and the
		// assertion below would then be measuring that race instead of adoption. Re-pin until it
		// sticks — once no apply is in flight, the value stays and the loop ends.
		Eventually(func(g Gomega) string {
			md := &capiv1beta2.MachineDeployment{}
			g.Expect(k8sClient.Get(suiteCtx, mdKey, md)).To(Succeed())
			if name := md.Spec.Template.Spec.Bootstrap.DataSecretName; name != nil && *name == legacyName {
				return *name
			}
			patched := md.DeepCopy()
			patched.Spec.Template.Spec.Bootstrap.DataSecretName = ptr(legacyName)
			g.Expect(k8sClient.Patch(suiteCtx, patched, client.MergeFrom(md))).To(Succeed())
			return ""
		}, 20*time.Second, 500*time.Millisecond).Should(Equal(legacyName))

		By("forcing a reconcile that re-applies the MachineDeployment")
		fresh := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: ng.Name}, fresh)).To(Succeed())
		bumped := fresh.DeepCopy()
		bumped.Annotations = map[string]string{"manual-rollout-id": "adopt-1"}
		Expect(k8sClient.Patch(suiteCtx, bumped, client.MergeFrom(fresh))).To(Succeed())

		By("the legacy name survives — no MachineSet replacement, no node roll")
		Consistently(func(g Gomega) {
			got := &capiv1beta2.MachineDeployment{}
			g.Expect(k8sClient.Get(suiteCtx, mdKey, got)).To(Succeed())
			g.Expect(got.Spec.Template.Spec.Bootstrap.DataSecretName).NotTo(BeNil())
			g.Expect(*got.Spec.Template.Spec.Bootstrap.DataSecretName).To(Equal(legacyName))
		}, 5*time.Second, 500*time.Millisecond).Should(Succeed())
	})
})
