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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The handover is the riskiest mechanism in this migration and the one a fake client models
// worst: server-side apply, field ownership and ForceOwnership are all decided by the API
// server. These specs run it against a real one.
var _ = Describe("CAPI cluster handover", Serial, func() {
	It("takes fields over from Helm's field manager and leaves its ownership metadata alone", func() {
		name := testenv.UniqueName("handover-credentials")
		helmOwned := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      name,
				"namespace": common.MachineNamespace,
				"labels": map[string]any{
					"app":              "provider-controller",
					helmManagedByLabel: "Helm",
				},
				"annotations": map[string]any{
					helmReleaseNameAnnotation: "node-manager",
				},
			},
			"data": map[string]any{"token": "b2xk"},
		}}
		Expect(k8sClient.Patch(suiteCtx, helmOwned, client.Apply, client.FieldOwner("helm"))).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, helmOwned))).To(Succeed())
		})

		desired := clusterHandoverTestObject("v1", "Secret", common.MachineNamespace, name)
		desired.SetLabels(map[string]string{"app": "provider-controller"})
		desired.Object["data"] = map[string]any{"token": "bmV3"}

		reconciler := &ClusterReconciler{}
		reconciler.Client = k8sClient
		reconciler.APIReader = k8sClient
		Expect(reconciler.applyClusterObject(suiteCtx, desired)).To(Succeed())

		applied := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, client.ObjectKey{Namespace: common.MachineNamespace, Name: name}, applied)).To(Succeed())
		Expect(applied.Data["token"]).To(Equal([]byte("new")), "a field Helm owns must be taken over, not conflicted on")
		Expect(applied.Annotations).To(HaveKeyWithValue("helm.sh/resource-policy", "keep"))
		Expect(applied.Labels).To(HaveKeyWithValue("module", "node-manager"))
		// Helm's ownership metadata stays: nothing needs it removed, and the keep annotation
		// is what stops the prune.
		Expect(applied.Labels).To(HaveKeyWithValue(helmManagedByLabel, "Helm"))
		Expect(applied.Annotations).To(HaveKeyWithValue(helmReleaseNameAnnotation, "node-manager"))

		var nodeControllerOwns bool
		for _, entry := range applied.ManagedFields {
			if entry.Manager == "node-controller" {
				nodeControllerOwns = true
			}
		}
		Expect(nodeControllerOwns).To(BeTrue(), "node-controller must end up as a field manager of the adopted object")
	})

	It("preserves the controlPlaneEndpoint the provider controller wrote", func() {
		name := testenv.UniqueName("handover-cluster")
		infrastructure := clusterHandoverTestObject(
			"infrastructure.cluster.x-k8s.io/v1alpha1", "DeckhouseCluster", common.MachineNamespace, name,
		)
		Expect(k8sClient.Create(suiteCtx, infrastructure)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, infrastructure))).To(Succeed())
		})

		live := infrastructure.DeepCopy()
		Expect(unstructured.SetNestedMap(
			live.Object,
			map[string]any{"host": "192.0.2.10", "port": int64(6443)},
			"spec", "controlPlaneEndpoint",
		)).To(Succeed())
		Expect(k8sClient.Update(suiteCtx, live)).To(Succeed())

		// What the contract renders: no endpoint at all, because only the provider knows it.
		desired := clusterHandoverTestObject(
			"infrastructure.cluster.x-k8s.io/v1alpha1", "DeckhouseCluster", common.MachineNamespace, name,
		)
		reconciler := &ClusterReconciler{}
		reconciler.Client = k8sClient
		reconciler.APIReader = k8sClient
		Expect(reconciler.applyClusterObject(suiteCtx, desired)).To(Succeed())

		applied := clusterHandoverTestObject(
			"infrastructure.cluster.x-k8s.io/v1alpha1", "DeckhouseCluster", common.MachineNamespace, name,
		)
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(applied), applied)).To(Succeed())
		host, found, err := unstructured.NestedString(applied.Object, "spec", "controlPlaneEndpoint", "host")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue(), "the endpoint the provider wrote must survive every apply")
		Expect(host).To(Equal("192.0.2.10"))

		By("a template that does render an endpoint overwrites the live one")
		// OpenStack renders it from the apiserver addresses this controller watches, so after a
		// master is replaced the rendered value is the current one and the live value is stale.
		rendered := clusterHandoverTestObject(
			"infrastructure.cluster.x-k8s.io/v1alpha1", "DeckhouseCluster", common.MachineNamespace, name,
		)
		Expect(unstructured.SetNestedMap(
			rendered.Object,
			map[string]any{"host": "192.0.2.20", "port": int64(6443)},
			"spec", "controlPlaneEndpoint",
		)).To(Succeed())
		Expect(reconciler.applyClusterObject(suiteCtx, rendered)).To(Succeed())

		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(applied), applied)).To(Succeed())
		host, _, err = unstructured.NestedString(applied.Object, "spec", "controlPlaneEndpoint", "host")
		Expect(err).NotTo(HaveOccurred())
		Expect(host).To(Equal("192.0.2.20"))
	})
})
