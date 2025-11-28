/*
Copyright 2025 Flant JSC

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

package controller

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cniswitcherv1alpha1 "deckhouse.io/cni-switch-helper/api/v1alpha1"
)

var _ = Describe("CNIMigration Controller", func() {
	const (
		NodeName         = "test-node"
		MigrationName    = "test-migration"
		PodName          = "test-pod"
		Namespace        = "default"
		CurrentCNI             = "flannel"
		EffectiveCNIAnnotation = "effective-cni.network.deckhouse.io"
		Timeout                = time.Second * 10
		Interval               = time.Millisecond * 250
	)

	BeforeEach(func() {
		// Set the NODE_NAME environment variable for the reconciler
		Expect(os.Setenv("NODE_NAME", NodeName)).To(Succeed())

		// Create a Node object
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeName,
			},
		}
		Expect(k8sClient.Create(context.Background(), node)).To(Succeed())
	})

	AfterEach(func() {
		// Clean up resources
		Expect(os.Unsetenv("NODE_NAME")).To(Succeed())
		// Delete all resources - simple cleanup for tests
		_ = k8sClient.DeleteAllOf(context.Background(), &cniswitcherv1alpha1.CNIMigration{})
		_ = k8sClient.DeleteAllOf(context.Background(), &cniswitcherv1alpha1.CNINodeMigration{})
		_ = k8sClient.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace(Namespace))
		_ = k8sClient.DeleteAllOf(context.Background(), &corev1.Node{})
	})

	Context("When reconciling during Prepare phase", func() {
		It("should annotate pods and update status", func() {
			ctx := context.Background()

			// 1. Create a test pod on the node
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PodName,
					Namespace: Namespace,
				},
				Spec: corev1.PodSpec{
					NodeName: NodeName,
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "nginx",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			
			// Manually update pod status to Running
			pod.Status.Phase = corev1.PodRunning
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())


			// 2. Create a CNIMigration resource to trigger reconciliation
			cniMigration := &cniswitcherv1alpha1.CNIMigration{
				ObjectMeta: metav1.ObjectMeta{
					Name: MigrationName,
				},
				Spec: cniswitcherv1alpha1.CNIMigrationSpec{
					Phase:     "Prepare",
					TargetCNI: "cilium",
				},
				Status: cniswitcherv1alpha1.CNIMigrationStatus{
					CurrentCNI: CurrentCNI,
				},
			}
			Expect(k8sClient.Create(ctx, cniMigration)).To(Succeed())

			// 3. Wait for CNINodeMigration to be created and then for its status to be updated
			nodeMigrationLookupKey := types.NamespacedName{Name: NodeName}
			createdNodeMigration := &cniswitcherv1alpha1.CNINodeMigration{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, nodeMigrationLookupKey, createdNodeMigration)
				return err == nil
			}, Timeout, Interval).Should(BeTrue(), "should create CNINodeMigration")

			// 4. Check that the pod gets annotated
			podLookupKey := types.NamespacedName{Name: PodName, Namespace: Namespace}
			annotatedPod := &corev1.Pod{}
			Eventually(func() (string, bool) {
				err := k8sClient.Get(ctx, podLookupKey, annotatedPod)
				if err != nil {
					return "", false
				}
				val, ok := annotatedPod.Annotations[EffectiveCNIAnnotation]
				return val, ok
			}, Timeout, Interval).Should(Equal(CurrentCNI), "pod should be annotated with the current CNI")

			// 5. Check that the CNINodeMigration status is updated to Prepared
			Eventually(func() string {
				err := k8sClient.Get(ctx, nodeMigrationLookupKey, createdNodeMigration)
				if err != nil {
					return ""
				}
				return createdNodeMigration.Status.Phase
			}, Timeout, Interval).Should(Equal("Prepared"), "CNINodeMigration phase should be Prepared")

			// 6. Check for the "PreparationSucceeded" condition
			var condition metav1.Condition
			for _, c := range createdNodeMigration.Status.Conditions {
				if c.Type == "PreparationSucceeded" {
					condition = c
					break
				}
			}
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("PodsAnnotated"))
		})
	})
})
