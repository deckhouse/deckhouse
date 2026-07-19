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

package nodeconfig

import (
	"context"
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// clusterDNSAddress is the ClusterIP the envtest apiserver assigned to the DNS
// service; the rendered config must point kubelet at it.
var clusterDNSAddress string

// apiServerEndpoints are the addresses envtest publishes for its apiserver.
var apiServerEndpoints []string

const (
	testKubernetesVersion = "1.35"
	testContainerdDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testCNIDigest         = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testKubeletDigest     = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	testClusterCA         = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
)

// User story: As a cluster operator, I want the nodes of an immutable NodeGroup
// to be configured from the NodeGroup I wrote, so that I manage immutable nodes
// through the same object as every other node group and never write per-node
// configuration by hand.
var _ = Describe("NodeConfig controller", func() {
	BeforeEach(func(ctx context.Context) {
		ensureClusterInputs(ctx)
	})

	It("renders a NodeConfig for a node of an immutable group", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-imm")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{
				MaxPods:              ptr.To[int32](150),
				ContainerLogMaxSize:  "100Mi",
				ContainerLogMaxFiles: ptr.To[int32](7),
			}
			ng.Spec.NodeTemplate = &deckhousev1.NodeTemplate{
				Labels: map[string]string{"role": "worker"},
			}
			ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
				ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
				Automatic: &deckhousev1.AutomaticDisruptionSpec{
					Windows: []deckhousev1.DisruptionWindow{{From: "03:00", To: "06:00", Days: []string{"Mon"}}},
				},
			}
		})
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)

			g.Expect(nc.Spec.NodeName).To(Equal(nodeName))
			g.Expect(nc.Spec.OSImage).NotTo(BeEmpty())

			// Kubelet settings come straight from the NodeGroup.
			g.Expect(nc.Spec.Kubelet.MaxPods).To(Equal(150))
			g.Expect(nc.Spec.Kubelet.ContainerLogMaxSize).To(Equal("100Mi"))
			g.Expect(nc.Spec.Kubelet.ContainerLogMaxFiles).To(Equal(7))
			g.Expect(nc.Spec.Kubelet.ClusterDNS).To(ConsistOf(clusterDNSAddress))
			g.Expect(nc.Spec.Kubelet.ClusterDomain).To(Equal("cluster.local"))
			g.Expect(nc.Spec.Kubelet.NodeLabels).To(HaveKeyWithValue(nodecommon.NodeGroupLabel, internalv1alpha1.NodeLabelValue(ngName)))
			g.Expect(nc.Spec.Kubelet.NodeLabels).To(HaveKeyWithValue("role", internalv1alpha1.NodeLabelValue("worker")))

			// kubelet loads the CA from a file on tmpfs, so the node rewrites
			// it from here on every boot. A config without it leaves a rebooted
			// node unable to start kubelet at all.
			g.Expect(nc.Spec.Kubelet.CACert).To(Equal(base64.StdEncoding.EncodeToString([]byte(testClusterCA))))

			// The node talks to the API servers the cluster actually has.
			g.Expect(nc.Spec.APIServerEndpoints).To(ConsistOf(apiServerEndpoints))

			// Every immutable node runs these three system extensions, pinned
			// by the digests of this release.
			g.Expect(nc.Spec.Extensions).To(HaveLen(3))
			byName := map[string]string{}
			for _, ext := range nc.Spec.Extensions {
				byName[ext.Name] = ext.Digest
			}
			g.Expect(byName).To(HaveKeyWithValue(containerdExtension, testContainerdDigest))
			g.Expect(byName).To(HaveKeyWithValue(cniExtension, testCNIDigest))
			g.Expect(byName).To(HaveKeyWithValue(kubeletExtension, testKubeletDigest))

			// The update window is the one the operator configured.
			g.Expect(nc.Spec.UpdatePolicy.Window.From).To(Equal("03:00"))
			g.Expect(nc.Spec.UpdatePolicy.Window.To).To(Equal("06:00"))

			g.Expect(nc.Spec.RegistryPackagesProxyAccessTokenB64).NotTo(BeEmpty())

			// This config replaces the bootstrap one wholesale, so what the
			// node was bootstrapped with has to survive in it: kubelet does
			// not start without kernel.panic, and the OS renders its hostname
			// from this spec on every boot.
			g.Expect(nc.Spec.Kernel.Sysctl).To(HaveKeyWithValue("kernel.panic", internalv1alpha1.SysctlValue("10")))
			g.Expect(nc.Spec.Kernel.Sysctl).To(HaveKeyWithValue("kernel.panic_on_oops", internalv1alpha1.SysctlValue("1")))
			g.Expect(nc.Spec.Network.Hostname).To(Equal(nodeName))
			g.Expect(nc.Spec.Kubelet.ExternalCloudProvider).To(BeTrue())

			// The object is owned by its node, so it is collected with it.
			g.Expect(nc.OwnerReferences).To(HaveLen(1))
			g.Expect(nc.OwnerReferences[0].Kind).To(Equal("Node"))
			g.Expect(nc.OwnerReferences[0].Name).To(Equal(nodeName))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("re-renders the nodes of a group when the group changes", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-imm")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](110)}
		})
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.Kubelet.MaxPods).To(Equal(110))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("raising maxPods on the NodeGroup")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.Kubelet.MaxPods).To(Equal(200))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("renders a NodeConfig for a group that has no status yet", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-fresh")
		createImmutableNodeGroup(ctx, ngName, nil)

		// A brand new group has no status.kubernetesVersion: the version has to
		// come from the cluster configuration, or the first node of a group
		// would never get its config.
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		ng.Status.KubernetesVersion = ""
		Expect(k8sClient.Status().Update(ctx, ng)).To(Succeed())

		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			byName := map[string]string{}
			for _, ext := range nc.Spec.Extensions {
				byName[ext.Name] = ext.Digest
			}
			g.Expect(byName).To(HaveKeyWithValue(kubeletExtension, testKubeletDigest))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want a change to a NodeGroup to reach
	// its immutable nodes a few at a time, so that one bad setting cannot take
	// the whole group down at once.
	It("rolls a NodeGroup change out to the group one node at a time", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-roll")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](110)}
		})
		first := testenv.UniqueName("node")
		second := testenv.UniqueName("node")
		createNode(ctx, first, ngName)
		createNode(ctx, second, ngName)

		// Both nodes are configured on arrival: a node without a config has
		// nothing to run on, so it never waits for a slot.
		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, first).Spec.Kubelet.MaxPods).To(Equal(110))
			g.Expect(getNodeConfig(ctx, g, second).Spec.Kubelet.MaxPods).To(Equal(110))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		reportApplied(ctx, first)
		reportApplied(ctx, second)

		By("raising maxPods on the NodeGroup")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		// One node takes the change; the other keeps the old config until the
		// first one reports back.
		Eventually(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, first, second)).To(Equal(1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, first, second)).To(Equal(1))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the updated node reporting the spec it was given")
		for _, name := range []string{first, second} {
			if getNodeConfig(ctx, Default, name).Spec.Kubelet.MaxPods == 200 {
				reportApplied(ctx, name)
			}
		}

		Eventually(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, first, second)).To(Equal(2))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want a node that has to restart
	// kubelet to apply its config to be drained first and to see that happening,
	// so that the workload leaves before the interruption and I can tell what is
	// being done to the node and why.
	It("asks to interrupt a node through a NodeOperation", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-disrupt")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		// A group of one is interrupted without a drain — there is nowhere for
		// its workload to go — so the group has to look bigger than that.
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		ng.Status.Nodes = 2
		Expect(k8sClient.Status().Update(ctx, ng)).To(Succeed())

		var generation int64
		Eventually(func(g Gomega) {
			generation = getNodeConfig(ctx, g, nodeName).Generation
			g.Expect(generation).NotTo(BeZero())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the agent reporting it cannot apply the config without an interruption")
		requestDisruption(ctx, nodeName, generation)

		// The answer is an operation naming the node and the revision it
		// covers — the same object an operator would create by hand.
		var op *v1alpha1.NodeOperation
		Eventually(func(g Gomega) {
			op = findOperation(ctx, g, nodeName)
			g.Expect(op).NotTo(BeNil())
			g.Expect(op.Spec.Type).To(Equal(v1alpha1.NodeOperationApproveDisruption))
			g.Expect(op.Spec.ConfigGeneration).To(HaveValue(Equal(generation)))
			g.Expect(op.Spec.Drain.Skip).To(BeFalse())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The eviction is an operation of its own rather than a side effect, so
		// it can be watched, and it belongs to the operation that needs it.
		var drain *v1alpha1.NodeOperation
		Eventually(func(g Gomega) {
			drain = findDrainOf(ctx, g, op.Name)
			g.Expect(drain).NotTo(BeNil())
			g.Expect(drain.Spec.NodeName).To(Equal(nodeName))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		node := &corev1.Node{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			g.Expect(node.Annotations).To(HaveKeyWithValue(nodecommon.DrainingAnnotation, "node-operation"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(findOperation(ctx, g, nodeName).Status.Phase).NotTo(Equal(v1alpha1.NodeOperationInProgress))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the drain finishing")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			node.Annotations[nodecommon.DrainedAnnotation] = "node-operation"
			node.Spec.Unschedulable = true
			g.Expect(k8sClient.Update(ctx, node)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The eviction finishing is what lets the parent hand the node over.
		Eventually(func(g Gomega) {
			g.Expect(findDrainOf(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))

			parent := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, parent)).To(Succeed())
			g.Expect(parent.Status.Phase).To(Equal(v1alpha1.NodeOperationInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node reporting the operation done")
		Eventually(func(g Gomega) {
			done := findOperation(ctx, g, nodeName)
			done.Status.Phase = v1alpha1.NodeOperationCompleted
			g.Expect(k8sClient.Status().Update(ctx, done)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The node goes back to the scheduler, and the finished operation stays
		// as the record of what happened.
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			g.Expect(node.Spec.Unschedulable).To(BeFalse())
			g.Expect(node.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As an operator, I want to drain an immutable node by creating
	// a NodeOperation, so that the workload leaves and the node stays out of
	// the scheduler until I say otherwise.
	It("completes a drain and leaves the node unschedulable", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-drain")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("drain")},
			Spec: v1alpha1.NodeOperationSpec{
				Type:     v1alpha1.NodeOperationDrain,
				NodeName: nodeName,
			},
		}
		Expect(k8sClient.Create(ctx, op)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		node := &corev1.Node{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			g.Expect(node.Annotations).To(HaveKeyWithValue(nodecommon.DrainingAnnotation, "node-operation"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the drain finishing")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			node.Annotations[nodecommon.DrainedAnnotation] = "node-operation"
			node.Spec.Unschedulable = true
			g.Expect(k8sClient.Update(ctx, node)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The eviction was the whole job: nobody has to carry anything out.
		Eventually(func(g Gomega) {
			done := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, done)).To(Succeed())
			g.Expect(done.Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// And the node stays where the operator put it.
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			g.Expect(node.Spec.Unschedulable).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node waiting for permission has not applied anything, whatever its
	// status claims. If the rollout took the claim at face value, the change
	// would walk through the whole group while every node sat waiting — exactly
	// what maxConcurrent exists to prevent.
	It("does not count a node waiting for a disruption as updated", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-wait")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](110)}
		})
		first := testenv.UniqueName("node")
		second := testenv.UniqueName("node")
		createNode(ctx, first, ngName)
		createNode(ctx, second, ngName)

		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, first).Spec.Kubelet.MaxPods).To(Equal(110))
			g.Expect(getNodeConfig(ctx, g, second).Spec.Kubelet.MaxPods).To(Equal(110))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		reportApplied(ctx, first)
		reportApplied(ctx, second)

		By("raising maxPods, and the node that gets it asking to be interrupted")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		var waiting string
		Eventually(func(g Gomega) {
			waiting = ""
			for _, name := range []string{first, second} {
				if getNodeConfig(ctx, g, name).Spec.Kubelet.MaxPods == 200 {
					waiting = name
				}
			}
			g.Expect(waiting).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The node reports the way the agent does while holding a config: the
		// generation it is still running, plus the request to interrupt it.
		nc := getNodeConfig(ctx, Default, waiting)
		heldGeneration := nc.Generation
		reportHeld(ctx, waiting, heldGeneration)

		// The other node must not be given the change while this one waits.
		Consistently(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, first, second)).To(Equal(1))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("leaves a manual group to the operator", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-manual")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{ApprovalMode: deckhousev1.DisruptionApprovalModeManual}
		})
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		var generation int64
		Eventually(func(g Gomega) {
			generation = getNodeConfig(ctx, g, nodeName).Generation
			g.Expect(generation).NotTo(BeZero())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		requestDisruption(ctx, nodeName, generation)

		// Nothing is created on the node's behalf: the operator decides.
		Consistently(func(g Gomega) {
			g.Expect(findOperation(ctx, g, nodeName)).To(BeNil())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("leaves nodes of a bashible-managed group alone", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-mutable")
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: ngName},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType:   deckhousev1.NodeTypeCloudEphemeral,
				SystemType: deckhousev1.SystemTypeMutable,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1,
					MaxPerZone: 3,
					ClassReference: deckhousev1.ClassReference{
						Kind: "DVPInstanceClass",
						Name: "worker",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, ng)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ng) })

		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Consistently(func(g Gomega) {
			nc := &internalv1alpha1.NodeConfig{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, nc)
			g.Expect(err).To(HaveOccurred())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("removes the NodeConfig when a node leaves the immutable group", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-imm")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		node := createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			getNodeConfig(ctx, g, nodeName)
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("dropping the node-group label from the node")
		fresh := &corev1.Node{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: node.Name}, fresh)).To(Succeed())
		patch := client.MergeFrom(fresh.DeepCopy())
		delete(fresh.Labels, nodecommon.NodeGroupLabel)
		Expect(k8sClient.Patch(ctx, fresh, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			nc := &internalv1alpha1.NodeConfig{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, nc)
			g.Expect(err).To(HaveOccurred())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})
})

// reportHeld is what the agent publishes while it holds a config it may not
// apply yet: the generation it is still running, and the request to interrupt.
func reportHeld(ctx context.Context, name string, heldGeneration int64) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		nc := getNodeConfig(ctx, g, name)
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               disruptionRequiredCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "KubeletRestartRequired",
			Message:            "applying this config restarts kubelet",
			ObservedGeneration: heldGeneration,
		})
		// Deliberately the held generation with a Ready phase — an agent that
		// overstates what it has applied. The rollout must not take a node's
		// word for it while that same status says the node is still waiting to
		// be interrupted.
		nc.Status.ObservedGeneration = heldGeneration
		nc.Status.Phase = phaseReady
		g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// findDrainOf returns the eviction a given operation spawned, located the way
// the controller locates it: by ownership, not by a name anyone could take.
func findDrainOf(ctx context.Context, g Gomega, parent string) *v1alpha1.NodeOperation {
	ops := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, ops)).To(Succeed())
	for i := range ops.Items {
		if ops.Items[i].Spec.Type != v1alpha1.NodeOperationDrain {
			continue
		}
		for _, owner := range ops.Items[i].OwnerReferences {
			if owner.Name == parent {
				return &ops.Items[i]
			}
		}
	}
	return nil
}

// findOperation returns the operation covering this node, if the controller
// asked for one.
func findOperation(ctx context.Context, g Gomega, nodeName string) *v1alpha1.NodeOperation {
	ops := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, ops)).To(Succeed())
	for i := range ops.Items {
		// The child Drain names the same node; the caller is after the
		// operation that asked for it.
		if ops.Items[i].Spec.NodeName == nodeName && ops.Items[i].Spec.Type != v1alpha1.NodeOperationDrain {
			return &ops.Items[i]
		}
	}
	return nil
}

// requestDisruption is what the agent does when the config it was given cannot
// be applied without restarting kubelet, containerd or the extensions.
func requestDisruption(ctx context.Context, name string, generation int64) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		nc := getNodeConfig(ctx, g, name)
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               disruptionRequiredCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "KubeletRestartRequired",
			Message:            "applying this config restarts kubelet",
			ObservedGeneration: generation,
		})
		g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// clearDisruption is what the agent does once it has applied the config.
func clearDisruption(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		nc := getNodeConfig(ctx, g, name)
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               disruptionRequiredCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "Applied",
			Message:            "config applied",
			ObservedGeneration: nc.Generation,
		})
		nc.Status.ObservedGeneration = nc.Generation
		nc.Status.Phase = phaseReady
		g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// reportApplied is what the node agent does after reconciling the spec it was
// given: the rollout waits for exactly this before moving on.
func reportApplied(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		nc := getNodeConfig(ctx, g, name)
		nc.Status.ObservedGeneration = nc.Generation
		nc.Status.Phase = phaseReady
		g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// updatedNodes counts how many of the named nodes already carry the new spec.
func updatedNodes(ctx context.Context, g Gomega, names ...string) int {
	updated := 0
	for _, name := range names {
		if getNodeConfig(ctx, g, name).Spec.Kubelet.MaxPods == 200 {
			updated++
		}
	}
	return updated
}

func getNodeConfig(ctx context.Context, g Gomega, name string) *internalv1alpha1.NodeConfig {
	nc := &internalv1alpha1.NodeConfig{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, nc)).To(Succeed())
	return nc
}

func createImmutableNodeGroup(ctx context.Context, name string, mutate func(*deckhousev1.NodeGroup)) *deckhousev1.NodeGroup {
	GinkgoHelper()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:   deckhousev1.NodeTypeCloudEphemeral,
			SystemType: deckhousev1.SystemTypeImmutable,
			// A CloudEphemeral group must declare how its nodes are ordered.
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				ClassReference: deckhousev1.ClassReference{
					Kind: "DVPInstanceClass",
					Name: "worker",
				},
			},
		},
	}
	if mutate != nil {
		mutate(ng)
	}
	Expect(k8sClient.Create(ctx, ng)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ng) })

	// The kubelet system extension is chosen by the group's Kubernetes version,
	// which the nodegroup-status controller normally fills in.
	fresh := &deckhousev1.NodeGroup{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, fresh)).To(Succeed())
	fresh.Status.KubernetesVersion = testKubernetesVersion
	Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

	return fresh
}

func createNode(ctx context.Context, name, ngName string) *corev1.Node {
	GinkgoHelper()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{nodecommon.NodeGroupLabel: ngName},
		},
	}
	Expect(k8sClient.Create(ctx, node)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })
	return node
}

// ensureClusterInputs creates the cluster state a NodeConfig is rendered from:
// an API server endpoint, the DNS service, the image digests of the release and
// the registry packages proxy token.
func ensureClusterInputs(ctx context.Context) {
	GinkgoHelper()

	ensureNamespace(ctx, kubeSystemNS)
	ensureNamespace(ctx, cloudInstanceManagerNS)

	// envtest publishes its own apiserver in the default/kubernetes
	// EndpointSlice; the rendered config must point the node at exactly that.
	slice := &discoveryv1.EndpointSlice{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, slice)).To(Succeed())
	apiServerEndpoints = nil
	for _, endpoint := range slice.Endpoints {
		for _, addr := range endpoint.Addresses {
			for _, port := range slice.Ports {
				if port.Name != nil && *port.Name == "https" && port.Port != nil {
					apiServerEndpoints = append(apiServerEndpoints, fmt.Sprintf("https://%s:%d", addr, *port.Port))
				}
			}
		}
	}
	Expect(apiServerEndpoints).NotTo(BeEmpty(), "envtest should publish its apiserver endpoint")

	// The envtest apiserver allocates ClusterIPs from its own service CIDR, so
	// the address is whatever it hands out; the suite remembers it to assert on.
	dnsService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeSystemNS,
			Name:      "kube-dns",
			Labels:    map[string]string{dnsAppLabel: "kube-dns"},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 53, Protocol: corev1.ProtocolUDP}},
		},
	}
	ensureObject(ctx, dnsService)
	fresh := &corev1.Service{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: "kube-dns"}, fresh)).To(Succeed())
	clusterDNSAddress = fresh.Spec.ClusterIP

	digests := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q}}`,
		testContainerdDigest, testCNIDigest, testKubeletDigest)
	ensureObject(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName},
		Data:       map[string]string{imagesDigestsKey: digests},
	})

	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: registryPackagesProxyTokenSecret},
		Data:       map[string][]byte{registryPackagesProxyTokenKey: []byte("proxy-token")},
	})

	// In a real cluster kube-controller-manager publishes this ConfigMap into
	// every namespace; envtest runs the apiserver alone, so the suite creates
	// it. Without the CA a node cannot start kubelet after a reboot, which is
	// why rendering refuses to proceed without it.
	ensureObject(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterCAConfigMap},
		Data:       map[string]string{clusterCAKey: testClusterCA},
	})

	// The cluster's own Kubernetes version: the group's status carries it only
	// once the group has nodes, so this is where the version comes from.
	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterConfigSecretName},
		Data: map[string][]byte{
			clusterConfigKey: []byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterDomain: cluster.local\nkubernetesVersion: \"" + testKubernetesVersion + ".6\"\n"),
		},
	})
}

func ensureNamespace(ctx context.Context, name string) {
	GinkgoHelper()
	ensureObject(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}})
}

// ensureObject creates the object unless the suite already created it.
func ensureObject(ctx context.Context, obj client.Object) {
	GinkgoHelper()
	err := k8sClient.Create(ctx, obj)
	if err != nil && !apierrorsIsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func apierrorsIsAlreadyExists(err error) bool {
	return apierrors.IsAlreadyExists(err)
}
