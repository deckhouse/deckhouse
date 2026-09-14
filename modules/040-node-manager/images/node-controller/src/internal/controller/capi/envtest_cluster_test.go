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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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

type staticClusterCreateRecorder struct {
	client.Client
	creates int
}

func (r *staticClusterCreateRecorder) Create(
	ctx context.Context,
	object client.Object,
	opts ...client.CreateOption,
) error {
	if object.GetName() == "static" && object.GetObjectKind().GroupVersionKind().Kind == "Cluster" {
		r.creates++
	}
	return r.Client.Create(ctx, object, opts...)
}

var _ = Describe("CAPI cluster reconciliation", Serial, func() {
	It("creates static cluster resources even when the provider cluster template is broken", func() {
		templateSecretKey := client.ObjectKey{
			Namespace: cloudprovider.ProviderTemplateSecretNamespace,
			Name:      "d8-cloud-provider-dvp-capi",
		}
		templateSecret := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, templateSecretKey, templateSecret)).To(Succeed())
		originalTemplateSecret := templateSecret.DeepCopy()
		templateSecret.Data[cloudprovider.CAPIClusterTemplateKey] = []byte("version: unsupported\ntemplate: broken\n")
		Expect(k8sClient.Update(suiteCtx, templateSecret)).To(Succeed())

		staticNodeGroup := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("static-cluster")},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType:        deckhousev1.NodeTypeStatic,
				StaticInstances: &deckhousev1.StaticInstancesSpec{},
			},
		}
		Expect(k8sClient.Create(suiteCtx, staticNodeGroup)).To(Succeed())

		staticCluster := clusterHandoverTestObject("cluster.x-k8s.io/v1beta2", "Cluster", common.MachineNamespace, "static")
		staticHealthCheck := clusterHandoverTestObject(
			"cluster.x-k8s.io/v1beta2", "MachineHealthCheck", common.MachineNamespace, "static-machine-health-check",
		)
		DeferCleanup(func() {
			currentTemplateSecret := &corev1.Secret{}
			if err := k8sClient.Get(suiteCtx, templateSecretKey, currentTemplateSecret); err == nil {
				currentTemplateSecret.Data = originalTemplateSecret.Data
				Expect(k8sClient.Update(suiteCtx, currentTemplateSecret)).To(Succeed())
			}

			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, staticNodeGroup))).To(Succeed())
			Eventually(func() bool {
				actual := &deckhousev1.NodeGroup{}
				return apierrors.IsNotFound(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(staticNodeGroup), actual))
			}, 30*time.Second, 250*time.Millisecond).Should(BeTrue())
			for _, object := range []*unstructured.Unstructured{staticCluster, staticHealthCheck} {
				Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, object))).To(Succeed())
			}
		})

		recordedClient := &staticClusterCreateRecorder{Client: k8sClient}
		reconciler := &ClusterReconciler{}
		reconciler.Client = recordedClient
		reconciler.APIReader = k8sClient
		_, err := reconciler.Reconcile(suiteCtx, ctrl.Request{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported cluster.yaml contract version"))
		Expect(recordedClient.creates).To(Equal(1), "the static path must run before the broken provider path returns")

		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(staticCluster), staticCluster)).To(Succeed())
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(staticHealthCheck), staticHealthCheck)).To(Succeed())
	})
})
