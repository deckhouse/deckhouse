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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The checks every cloud MachineHealthCheck must end up with, written out rather than taken from
// the constants: the values are the contract, and a spec built from the constants would follow
// any change to them.
var (
	wantCloudHealthChecks = map[string]any{
		"nodeStartupTimeoutSeconds": int64(1200),
		"unhealthyNodeConditions": []any{
			map[string]any{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(300)},
			map[string]any{"type": "Ready", "status": "False", "timeoutSeconds": int64(600)},
		},
	}
	wantCloudHealthCheckRemediation = map[string]any{
		"triggerIf": map[string]any{"unhealthyLessThanOrEqualTo": "50%"},
	}
)

// legacyCloudMachineHealthCheck is the object node-controller created before the checks were
// fixed: Ready=False at 5 minutes and no remediation limit.
func legacyCloudMachineHealthCheck(clusterName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineHealthCheck",
		"metadata": map[string]any{
			"name":      clusterName + "-machine-health-check",
			"namespace": common.MachineNamespace,
			"labels": map[string]any{
				"heritage": "deckhouse",
				"module":   "node-manager",
				"app":      "capi-controller-manager",
			},
		},
		"spec": map[string]any{
			"clusterName": clusterName,
			"selector": map[string]any{
				"matchLabels": map[string]any{"cluster.x-k8s.io/cluster-name": clusterName},
			},
			"checks": map[string]any{
				"nodeStartupTimeoutSeconds": int64(1200),
				"unhealthyNodeConditions": []any{
					map[string]any{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(300)},
					map[string]any{"type": "Ready", "status": "False", "timeoutSeconds": int64(300)},
				},
			},
		},
	}}
}

func expectCloudHealthCheckSpec(healthCheck *unstructured.Unstructured) {
	GinkgoHelper()

	checks, _, err := unstructured.NestedMap(healthCheck.Object, "spec", "checks")
	Expect(err).NotTo(HaveOccurred())
	Expect(checks).To(Equal(wantCloudHealthChecks))
	remediation, _, err := unstructured.NestedMap(healthCheck.Object, "spec", "remediation")
	Expect(err).NotTo(HaveOccurred())
	Expect(remediation).To(Equal(wantCloudHealthCheckRemediation))
}

// fieldManagers lists the managedFields entries as manager/operation: one manager name can own
// fields through Update and Apply at once, and the two are separate owners to the apiserver.
func fieldManagers(object client.Object) []string {
	managers := make([]string, 0, len(object.GetManagedFields()))
	for _, entry := range object.GetManagedFields() {
		managers = append(managers, entry.Manager+"/"+string(entry.Operation))
	}
	return managers
}

var _ = Describe("CAPI cloud MachineHealthCheck", Serial, func() {
	var (
		reconciler    *ClusterReconciler
		clusterConfig common.ClusterConfiguration
	)

	BeforeEach(func() {
		reconciler = &ClusterReconciler{}
		reconciler.Client = k8sClient
		reconciler.APIReader = k8sClient

		var err error
		clusterConfig, err = common.ReadClusterConfiguration(suiteCtx, k8sClient)
		Expect(err).NotTo(HaveOccurred())
	})

	// The suite's manager runs this reconciler too and keeps the "dvp" MachineHealthCheck applied,
	// so a spec that plants an old object there would race it. A cluster name of the spec's own is
	// never touched by the manager.
	ensureCloudCluster := func(clusterName string) *unstructured.Unstructured {
		GinkgoHelper()

		registration := cloudprovider.Registration{
			Type:                  "dvp",
			CAPIClusterName:       clusterName,
			CAPIClusterKind:       "DeckhouseCluster",
			CAPIClusterAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
		}
		Expect(reconciler.ensureCommonCloudClusterResources(suiteCtx, registration, clusterConfig)).To(Succeed())

		healthCheck := clusterHandoverTestObject(
			"cluster.x-k8s.io/v1beta2", "MachineHealthCheck", common.MachineNamespace, clusterName+"-machine-health-check",
		)
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(healthCheck), healthCheck)).To(Succeed())
		return healthCheck
	}

	cleanupCloudCluster := func(clusterName string) {
		DeferCleanup(func() {
			for _, object := range []*unstructured.Unstructured{
				clusterHandoverTestObject("cluster.x-k8s.io/v1beta2", "MachineHealthCheck",
					common.MachineNamespace, clusterName+"-machine-health-check"),
				clusterHandoverTestObject("cluster.x-k8s.io/v1beta2", "Cluster", common.MachineNamespace, clusterName),
				clusterHandoverTestObject("infrastructure.cluster.x-k8s.io/v1alpha1", "DeckhouseControlPlane",
					common.MachineNamespace, clusterName+"-control-plane"),
			} {
				Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, object))).To(Succeed())
			}
		})
	}

	It("applies the fixed checks and the remediation limit to the provider cluster on reconcile", func() {
		dvpHealthCheck := clusterHandoverTestObject(
			"cluster.x-k8s.io/v1beta2", "MachineHealthCheck", common.MachineNamespace, "dvp-machine-health-check",
		)
		// The manager applied it at start. Deleted first, the object read below comes from a fresh
		// apply rather than from the suite start.
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, dvpHealthCheck))).To(Succeed())

		_, err := reconciler.Reconcile(suiteCtx, ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(dvpHealthCheck), dvpHealthCheck)).To(Succeed())
		expectCloudHealthCheckSpec(dvpHealthCheck)
		Expect(fieldManagers(dvpHealthCheck)).To(ContainElement("node-controller/Apply"))
	})

	It("creates a missing MachineHealthCheck with the fixed checks", func() {
		clusterName := testenv.UniqueName("mhc-fresh")
		cleanupCloudCluster(clusterName)

		healthCheck := ensureCloudCluster(clusterName)
		expectCloudHealthCheckSpec(healthCheck)
		Expect(healthCheck.GetLabels()).To(HaveKeyWithValue("app", "capi-controller-manager"))
		Expect(healthCheck.GetAnnotations()).To(HaveKeyWithValue("helm.sh/resource-policy", "keep"))
	})

	It("updates the MachineHealthCheck node-controller created with Create before the fix", func() {
		clusterName := testenv.UniqueName("mhc-created")
		cleanupCloudCluster(clusterName)

		// Exactly what the former createIfNotExists sent. On a live cluster its managedFields show
		// manager node-controller with operation Update, which is a different field manager from
		// node-controller's Apply.
		legacy := legacyCloudMachineHealthCheck(clusterName)
		Expect(k8sClient.Create(suiteCtx, legacy, client.FieldOwner("node-controller"))).To(Succeed())
		Expect(fieldManagers(legacy)).To(ConsistOf("node-controller/Update"))

		healthCheck := ensureCloudCluster(clusterName)
		expectCloudHealthCheckSpec(healthCheck)
		Expect(fieldManagers(healthCheck)).To(ContainElement("node-controller/Apply"))
	})

	It("takes over the MachineHealthCheck Helm created before 1.77", func() {
		clusterName := testenv.UniqueName("mhc-helm")
		cleanupCloudCluster(clusterName)

		// CAPI's v1beta1 webhook defaulted maxUnhealthy to 100%, so a Helm-created object carries the
		// limit too, converted to remediation.triggerIf, and Helm owns it.
		legacy := legacyCloudMachineHealthCheck(clusterName)
		Expect(unstructured.SetNestedField(legacy.Object, "100%",
			"spec", "remediation", "triggerIf", "unhealthyLessThanOrEqualTo")).To(Succeed())
		Expect(k8sClient.Patch(suiteCtx, legacy, client.Apply, client.FieldOwner("helm"))).To(Succeed())

		healthCheck := ensureCloudCluster(clusterName)
		expectCloudHealthCheckSpec(healthCheck)
		Expect(fieldManagers(healthCheck)).To(ContainElement("node-controller/Apply"))
	})

	It("leaves an existing static MachineHealthCheck alone", func() {
		// Created before the static NodeGroup, so the manager's own reconcile of that NodeGroup
		// finds it too and cannot create it first.
		staticHealthCheck := clusterHandoverTestObject(
			"cluster.x-k8s.io/v1beta2", "MachineHealthCheck", common.MachineNamespace, "static-machine-health-check",
		)
		staticHealthCheck.Object["spec"] = map[string]any{
			"clusterName": "static",
			"selector": map[string]any{
				"matchLabels": map[string]any{"cluster.x-k8s.io/cluster-name": "static"},
			},
			"checks": map[string]any{"nodeStartupTimeoutSeconds": int64(600)},
		}
		Expect(k8sClient.Create(suiteCtx, staticHealthCheck)).To(Succeed())

		staticNodeGroup := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("static-mhc")},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType:        deckhousev1.NodeTypeStatic,
				StaticInstances: &deckhousev1.StaticInstancesSpec{},
			},
		}
		Expect(k8sClient.Create(suiteCtx, staticNodeGroup)).To(Succeed())

		staticCluster := clusterHandoverTestObject("cluster.x-k8s.io/v1beta2", "Cluster", common.MachineNamespace, "static")
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, staticNodeGroup))).To(Succeed())
			Eventually(func() bool {
				actual := &deckhousev1.NodeGroup{}
				return apierrors.IsNotFound(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(staticNodeGroup), actual))
			}, 30*time.Second, 250*time.Millisecond).Should(BeTrue())
			for _, object := range []*unstructured.Unstructured{staticCluster, staticHealthCheck} {
				Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, object))).To(Succeed())
			}
		})

		Expect(reconciler.ensureStaticCluster(suiteCtx, clusterConfig)).To(Succeed())

		actual := clusterHandoverTestObject(
			"cluster.x-k8s.io/v1beta2", "MachineHealthCheck", common.MachineNamespace, "static-machine-health-check",
		)
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(actual), actual)).To(Succeed())
		checks, _, err := unstructured.NestedMap(actual.Object, "spec", "checks")
		Expect(err).NotTo(HaveOccurred())
		Expect(checks).To(Equal(map[string]any{"nodeStartupTimeoutSeconds": int64(600)}),
			"the static MachineHealthCheck is created once and never overwritten")
		_, found, err := unstructured.NestedMap(actual.Object, "spec", "remediation")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})
})
