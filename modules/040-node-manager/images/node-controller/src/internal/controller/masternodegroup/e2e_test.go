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
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout = testenv.EventuallyTimeout
	eventuallyPoll    = testenv.EventuallyPoll

	// Pinned rather than read from the code: the name is stored on the object in every cluster, and
	// a renamed finalizer would hold the group of an upgraded cluster in deletion forever.
	masterFinalizer = "node-manager.deckhouse.io/master-node-group"
	// otherFinalizer stands for a finalizer of another controller on NodeGroup master.
	otherFinalizer = "test.deckhouse.io/hold"
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

// deleteMaster lets NodeGroup master go the way an operator does: the finalizer is stripped first,
// or the deletion would wait for a controller that the direct specs do not run.
func deleteMaster() {
	ng := &deckhousev1.NodeGroup{}
	err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: masterNodeGroupName}, ng)
	if apierrors.IsNotFound(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
	testenv.RemoveFinalizers(suiteCtx, k8sClient, ng)
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
var _ = Describe("MasterNodeGroup controller", Ordered, func() {
	BeforeAll(func() {
		deleteMaster()
		startManager()
	})

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

		Expect(master.Finalizers).To(ConsistOf(masterFinalizer), "the group is created protected")
	})

	// User story: As a cluster operator, I want NodeGroup master back with my own settings when it is
	// deleted by mistake, so that the control-plane nodes never lose the group that describes them.
	It("restores a deleted master NodeGroup with its own spec", func() {
		Eventually(masterExists, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		By("an operator tuning the group away from the template")
		master := getMaster()
		Expect(master.Finalizers).To(ContainElement(masterFinalizer))
		master.Labels = map[string]string{"example.com/team": "platform"}
		master.Annotations = map[string]string{"example.com/note": "tuned by hand"}
		master.Spec.NodeTemplate.Labels["example.com/role"] = "control-plane"
		master.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
			ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
			Automatic:    &deckhousev1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr.To(false)},
		}
		Expect(k8sClient.Update(suiteCtx, master)).To(Succeed())
		tuned := getMaster()

		By("deleting the group")
		Expect(k8sClient.Delete(suiteCtx, tuned)).To(Succeed())

		restored := &deckhousev1.NodeGroup{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: masterNodeGroupName}, restored)).To(Succeed())
			g.Expect(restored.UID).NotTo(Equal(tuned.UID))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		Expect(restored.DeletionTimestamp).To(BeNil())
		Expect(restored.Spec).To(Equal(tuned.Spec))
		Expect(restored.Labels).To(Equal(tuned.Labels))
		Expect(restored.Annotations).To(Equal(tuned.Annotations))
		Expect(restored.Finalizers).To(ConsistOf(masterFinalizer))

		Eventually(func(g Gomega) {
			events := &corev1.EventList{}
			g.Expect(k8sClient.List(suiteCtx, events)).To(Succeed())
			g.Expect(events.Items).To(ContainElement(SatisfyAll(
				HaveField("InvolvedObject.UID", restored.UID),
				HaveField("Type", corev1.EventTypeWarning),
				HaveField("Reason", "RestoredAfterDeletion"),
			)))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	// Another finalizer, foregroundDeletion of a cascading delete for one, keeps the deleted group
	// after the finalizer of this controller is gone, and a group created meanwhile would collide.
	It("restores a deleted master NodeGroup only after another finalizer lets it go", func() {
		Eventually(masterExists, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		master := getMaster()
		master.Finalizers = append(master.Finalizers, otherFinalizer)
		master.Spec.NodeTemplate.Labels["example.com/held"] = "true"
		Expect(k8sClient.Update(suiteCtx, master)).To(Succeed())
		held := getMaster()

		By("deleting the group while the other finalizer holds it")
		Expect(k8sClient.Delete(suiteCtx, held)).To(Succeed())
		Consistently(func(g Gomega) {
			current := &deckhousev1.NodeGroup{}
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: masterNodeGroupName}, current)).To(Succeed())
			g.Expect(current.UID).To(Equal(held.UID))
			g.Expect(current.Finalizers).To(ContainElement(masterFinalizer), "the spec stays on the old object")
		}, testenv.NegativeCheckDuration, eventuallyPoll).Should(Succeed())

		By("the other finalizer letting the group go")
		current := getMaster()
		controllerutil.RemoveFinalizer(current, otherFinalizer)
		Expect(k8sClient.Update(suiteCtx, current)).To(Succeed())

		restored := &deckhousev1.NodeGroup{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: masterNodeGroupName}, restored)).To(Succeed())
			g.Expect(restored.UID).NotTo(Equal(held.UID))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		Expect(restored.Spec).To(Equal(held.Spec))
		Expect(restored.Finalizers).To(ConsistOf(masterFinalizer))
	})
})

// These specs drive Reconcile directly: no manager runs while they do.
var _ = Describe("MasterNodeGroup controller reconcile", func() {
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

	// dhctl creates the group itself at bootstrap, without the finalizer.
	It("adds the finalizer to an existing master NodeGroup and never overwrites its spec", func() {
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
		Expect(master.Finalizers).To(ConsistOf(masterFinalizer))
	})
})

const (
	// deletionPolicyTemplate is the module template, relative to this package.
	deletionPolicyTemplate = "../../../../../../templates/master_node_group_deletion_policy.yaml"
	deletionPolicyName     = "master-node-group-deletion.deckhouse.io"
	deletionRefusal        = "NodeGroup master cannot be deleted"
)

// User story: As a cluster operator, I want the API server to refuse deleting NodeGroup master, so
// that one mistaken command cannot take away the group the control-plane nodes belong to.
var _ = Describe("The policy that keeps NodeGroup master", Ordered, func() {
	BeforeAll(func() {
		if !masterExists() {
			master := &deckhousev1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: masterNodeGroupName},
				Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic},
			}
			Expect(k8sClient.Create(suiteCtx, master)).To(Succeed())
		}
		installDeletionPolicy()
	})

	It("refuses to delete NodeGroup master", func() {
		Expect(k8sClient.Delete(suiteCtx, getMaster())).To(MatchError(ContainSubstring(deletionRefusal)))
		Expect(getMaster().DeletionTimestamp).To(BeNil())
	})

	It("lets any other NodeGroup be deleted", func() {
		worker := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("worker")},
			Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic},
		}
		Expect(k8sClient.Create(suiteCtx, worker)).To(Succeed())
		Expect(k8sClient.Delete(suiteCtx, worker)).To(Succeed())
	})
})

// installDeletionPolicy applies the policy and the binding of the module template until the calling
// container ends. Only the module labels are templated in them, so the Helm lines are dropped.
func installDeletionPolicy() {
	raw, err := os.ReadFile(deletionPolicyTemplate)
	Expect(err).NotTo(HaveOccurred())
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.ReplaceAll(line, "{{ $policyName }}", deletionPolicyName)
		if !strings.Contains(line, "{{") {
			lines = append(lines, line)
		}
	}
	docs := strings.Split(strings.Join(lines, "\n"), "\n---\n")
	Expect(len(docs)).To(BeNumerically(">=", 2))

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	Expect(yaml.UnmarshalStrict([]byte(docs[len(docs)-2]), policy)).To(Succeed())
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
	Expect(yaml.UnmarshalStrict([]byte(docs[len(docs)-1]), binding)).To(Succeed())
	Expect(k8sClient.Create(suiteCtx, policy)).To(Succeed())
	Expect(k8sClient.Create(suiteCtx, binding)).To(Succeed())

	// The API server loads and drops a policy asynchronously; a dry-run delete tells when.
	dryRunDelete := func() error { return k8sClient.Delete(suiteCtx, getMaster(), client.DryRunAll) }
	Eventually(dryRunDelete, time.Minute, eventuallyPoll).Should(MatchError(ContainSubstring(deletionRefusal)))

	DeferCleanup(func() {
		Expect(k8sClient.Delete(suiteCtx, binding)).To(Succeed())
		Expect(k8sClient.Delete(suiteCtx, policy)).To(Succeed())
		Eventually(dryRunDelete, time.Minute, eventuallyPoll).Should(Succeed())
	})
}
