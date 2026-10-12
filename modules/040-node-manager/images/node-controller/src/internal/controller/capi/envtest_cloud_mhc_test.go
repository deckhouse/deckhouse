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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/testenv"
)

var _ = Describe("cloud MachineHealthCheck apply", func() {
	var (
		clusterName string
		labels      map[string]interface{}
		reconciler  *ClusterReconciler
	)

	// legacyCloudMHC is the MHC node-controller created before the fix: Ready=Unknown and
	// Ready=False both at 5 minutes and no remediation limit.
	legacyCloudMHC := func() *unstructured.Unstructured {
		mhc := cloudMachineHealthCheck(clusterName, labels)
		Expect(unstructured.SetNestedSlice(mhc.Object, []interface{}{
			map[string]interface{}{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(300)},
			map[string]interface{}{"type": "Ready", "status": "False", "timeoutSeconds": int64(300)},
		}, "spec", "checks", "unhealthyNodeConditions")).To(Succeed())
		unstructured.RemoveNestedField(mhc.Object, "spec", "remediation")
		return mhc
	}

	getMHC := func() *unstructured.Unstructured {
		mhc := newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineHealthCheck")
		Expect(k8sClient.Get(suiteCtx, client.ObjectKey{
			Namespace: capiNamespace,
			Name:      clusterName + "-machine-health-check",
		}, mhc)).To(Succeed())
		return mhc
	}

	expectFixedSpec := func(mhc *unstructured.Unstructured) {
		Expect(mhc.Object).To(HaveKeyWithValue("spec", And(
			HaveKeyWithValue("clusterName", clusterName),
			HaveKeyWithValue("selector", HaveKeyWithValue("matchLabels",
				HaveKeyWithValue("cluster.x-k8s.io/cluster-name", clusterName))),
			HaveKeyWithValue("checks", And(
				HaveKeyWithValue("nodeStartupTimeoutSeconds", BeNumerically("==", 1200)),
				HaveKeyWithValue("unhealthyNodeConditions", HaveExactElements(
					And(
						HaveKeyWithValue("type", "Ready"),
						HaveKeyWithValue("status", "Unknown"),
						HaveKeyWithValue("timeoutSeconds", BeNumerically("==", 300)),
					),
					And(
						HaveKeyWithValue("type", "Ready"),
						HaveKeyWithValue("status", "False"),
						HaveKeyWithValue("timeoutSeconds", BeNumerically("==", 600)),
					),
				)),
			)),
			HaveKeyWithValue("remediation", HaveKeyWithValue("triggerIf",
				HaveKeyWithValue("unhealthyLessThanOrEqualTo", "50%"))),
		)))
		Expect(mhc.GetLabels()).To(HaveKeyWithValue("app", "capi-controller-manager"))
	}

	expectManager := func(mhc *unstructured.Unstructured, manager string, operation metav1.ManagedFieldsOperationType) {
		Expect(mhc.GetManagedFields()).To(ContainElement(And(
			HaveField("Manager", manager),
			HaveField("Operation", operation),
		)))
	}

	BeforeEach(func() {
		clusterName = testenv.UniqueName("mhc")
		labels = map[string]interface{}{
			"heritage": "deckhouse",
			"module":   "node-manager",
			"app":      "capi-controller-manager",
		}
		reconciler = &ClusterReconciler{}
		reconciler.Client = k8sClient

		DeferCleanup(func() {
			mhc := newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineHealthCheck")
			mhc.SetNamespace(capiNamespace)
			mhc.SetName(clusterName + "-machine-health-check")
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, mhc))).To(Succeed())
		})
	})

	It("creates the MHC with the bootstrap-tolerant checks and the remediation limit", func() {
		Expect(reconciler.applyCloudMachineHealthCheck(suiteCtx, clusterName, labels)).To(Succeed())

		mhc := getMHC()
		expectFixedSpec(mhc)
		expectManager(mhc, "node-controller", metav1.ManagedFieldsOperationApply)
	})

	It("updates an MHC that node-controller created with Create before the fix", func() {
		Expect(k8sClient.Create(suiteCtx, legacyCloudMHC(), client.FieldOwner("node-controller"))).To(Succeed())
		legacy := getMHC()
		expectManager(legacy, "node-controller", metav1.ManagedFieldsOperationUpdate)
		Expect(legacy.Object).To(HaveKeyWithValue("spec", Not(HaveKey("remediation"))))

		Expect(reconciler.applyCloudMachineHealthCheck(suiteCtx, clusterName, labels)).To(Succeed())

		mhc := getMHC()
		expectFixedSpec(mhc)
		expectManager(mhc, "node-controller", metav1.ManagedFieldsOperationApply)
	})

	It("takes over an MHC that Helm applied in 1.76", func() {
		helmMHC := legacyCloudMHC()
		Expect(unstructured.SetNestedField(helmMHC.Object, "100%",
			"spec", "remediation", "triggerIf", "unhealthyLessThanOrEqualTo")).To(Succeed())
		helmMHC.SetAnnotations(map[string]string{"helm.sh/resource-policy": "keep"})
		Expect(k8sClient.Patch(suiteCtx, helmMHC, client.Apply, client.FieldOwner("helm"))).To(Succeed())
		legacy := getMHC()
		Expect(legacy.GetManagedFields()).NotTo(ContainElement(HaveField("Manager", "node-controller")))

		Expect(reconciler.applyCloudMachineHealthCheck(suiteCtx, clusterName, labels)).To(Succeed())

		mhc := getMHC()
		expectFixedSpec(mhc)
		expectManager(mhc, "node-controller", metav1.ManagedFieldsOperationApply)
		// Fields node-controller does not apply stay with Helm.
		Expect(mhc.GetAnnotations()).To(HaveKeyWithValue("helm.sh/resource-policy", "keep"))
	})
})
