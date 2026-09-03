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

package masternodegroup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout = testenv.EventuallyTimeout
	eventuallyPoll    = testenv.EventuallyPoll
)

// reconcileMaster drives the reconciler with the live envtest client. Production injects the
// manager cache instead, so a NodeGroup cache scope narrower than cluster-wide is not covered here.
func reconcileMaster() (ctrl.Result, error) {
	r := &Reconciler{}
	r.InjectClient(k8sClient)
	return r.Reconcile(suiteCtx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: masterNodeGroupName},
	})
}

func getMaster() *deckhousev1.NodeGroup {
	ng := &deckhousev1.NodeGroup{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: masterNodeGroupName}, ng)).To(Succeed())
	return ng
}

func masterExists() bool {
	err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: masterNodeGroupName}, &deckhousev1.NodeGroup{})
	if apierrors.IsNotFound(err) {
		return false
	}
	Expect(err).NotTo(HaveOccurred())
	return true
}

func deleteMaster() {
	ng := &deckhousev1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: masterNodeGroupName}}
	Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, ng))).To(Succeed())
	Eventually(masterExists, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
}

// The suite shares one cluster-configuration Secret, created as a Cloud cluster in BeforeSuite;
// every spec that touches it restores that state, so the specs do not depend on their order.
var clusterConfigRef = types.NamespacedName{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName}

func setClusterConfig(data map[string][]byte) {
	DeferCleanup(restoreClusterConfig)
	secret := &corev1.Secret{}
	Expect(k8sClient.Get(suiteCtx, clusterConfigRef, secret)).To(Succeed())
	secret.Data = data
	Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
}

func deleteClusterConfigSecret() {
	DeferCleanup(restoreClusterConfig)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigRef.Namespace, Name: clusterConfigRef.Name}}
	Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, secret))).To(Succeed())
}

func restoreClusterConfig() {
	cloud := map[string][]byte{clusterConfigKey: []byte("clusterType: Cloud\n")}
	secret := &corev1.Secret{}
	err := k8sClient.Get(suiteCtx, clusterConfigRef, secret)
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigRef.Namespace, Name: clusterConfigRef.Name},
			Data:       cloud,
		}
		Expect(k8sClient.Create(suiteCtx, secret)).To(Succeed())
		return
	}
	Expect(err).NotTo(HaveOccurred())
	secret.Data = cloud
	Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
}

// User story: As a user bootstrapping a cluster, I want the `master` NodeGroup to exist without
// creating it by hand, so that the control-plane nodes are described in the API from the start and
// my own edits to that group are never overwritten.
var _ = Describe("MasterNodeGroup controller", func() {
	It("creates the master NodeGroup at startup on a cloud cluster", func() {
		// No manual reconcile here: this is the controller's own one-shot startup enqueue.
		Eventually(masterExists, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		master := getMaster()
		Expect(master.Spec.NodeType).To(Equal(deckhousev1.NodeTypeCloudPermanent))

		Expect(master.Spec.NodeTemplate).NotTo(BeNil())
		Expect(master.Spec.NodeTemplate.Labels).To(Equal(map[string]string{
			"node-role.kubernetes.io/control-plane": "",
			"node-role.kubernetes.io/master":        "",
		}))
		Expect(master.Spec.NodeTemplate.Taints).To(Equal([]corev1.Taint{{
			Key:    "node-role.kubernetes.io/control-plane",
			Effect: corev1.TaintEffectNoSchedule,
		}}))

		Expect(master.Spec.Disruptions).NotTo(BeNil())
		Expect(master.Spec.Disruptions.ApprovalMode).To(Equal(deckhousev1.DisruptionApprovalModeManual))
	})

	It("creates a Static master NodeGroup on a static cluster", func() {
		deleteMaster()
		setClusterConfig(map[string][]byte{clusterConfigKey: []byte("clusterType: Static\n")})

		_, err := reconcileMaster()
		Expect(err).NotTo(HaveOccurred())

		Expect(getMaster().Spec.NodeType).To(Equal(deckhousev1.NodeTypeStatic))
	})

	// The node type is not guessable: a CloudPermanent master group on a static cluster is wrong,
	// so an unreadable cluster configuration must leave the group absent and retry.
	It("creates nothing when the cluster configuration secret has no cluster-configuration.yaml", func() {
		deleteMaster()
		setClusterConfig(map[string][]byte{"unrelated-key": []byte("clusterType: Static\n")})

		_, err := reconcileMaster()
		Expect(err).To(HaveOccurred())

		Expect(masterExists()).To(BeFalse())
	})

	It("creates nothing when the cluster configuration carries no clusterType", func() {
		deleteMaster()
		setClusterConfig(map[string][]byte{clusterConfigKey: []byte("clusterDomain: cluster.local\n")})

		_, err := reconcileMaster()
		Expect(err).To(HaveOccurred())

		Expect(masterExists()).To(BeFalse())
	})

	It("creates nothing when the cluster configuration secret is absent", func() {
		deleteMaster()
		deleteClusterConfigSecret()

		_, err := reconcileMaster()
		Expect(err).To(HaveOccurred())

		Expect(masterExists()).To(BeFalse())
	})

	It("never overwrites an existing master NodeGroup", func() {
		deleteMaster()

		userOwned := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: masterNodeGroupName},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeStatic,
				Disruptions: &deckhousev1.DisruptionsSpec{
					ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
				},
			},
		}
		Expect(k8sClient.Create(suiteCtx, userOwned)).To(Succeed())

		_, err := reconcileMaster()
		Expect(err).NotTo(HaveOccurred())

		master := getMaster()
		Expect(master.Spec.Disruptions.ApprovalMode).To(Equal(deckhousev1.DisruptionApprovalModeAutomatic))
		Expect(master.Spec.NodeTemplate).To(BeNil())
	})
})
