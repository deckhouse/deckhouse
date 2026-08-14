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
	"sort"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// clusterDNSAddress is the ClusterIP the envtest apiserver assigned to the DNS
// service; the rendered config must point kubelet at it.
var clusterDNSAddress string

// apiServerEndpoints are the addresses envtest publishes for its apiserver.
var apiServerEndpoints []string

// apiServerPodIP is the address of the extra kube-apiserver pod one spec adds;
// it is outside the addresses envtest publishes for its own apiserver.
const apiServerPodIP = "10.99.0.7"

// terminatingPodFinalizer holds a deleted pod in Terminating, since nothing
// finishes a deletion here — there is no kubelet.
const terminatingPodFinalizer = "nodeconfig.test.deckhouse.io/keep-terminating"

const (
	testKubernetesVersion = "1.35"
	testContainerdDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	// testContainerdRebuiltDigest is what a later release ships instead.
	testContainerdRebuiltDigest = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	testPauseDigest             = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	testRegistryAddress         = "registry.example.com"
	testRegistryPath            = "/deckhouse/ce"
	testRegistryAuth            = "dXNlcjpwYXNzd29yZA=="
	testCNIDigest               = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testKubeletDigest           = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	testNodeletDigest           = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	testOSImageDigest           = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	testClusterCA               = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
	// The extension one spec asks for through a NodeExtensionRequest, and the
	// digest it is rebuilt under.
	testRequestedSysextName          = "custom-ext"
	testRequestedSysextDigest        = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	testRequestedSysextRebuiltDigest = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
)

// extensionDigests indexes a rendered config's extensions by name.
func extensionDigests(nc *internalv1alpha1.NodeConfig) map[string]string {
	byName := map[string]string{}
	for _, ext := range nc.Spec.Extensions {
		byName[ext.Name] = ext.Digest
	}
	return byName
}

// User story: As a cluster operator, I want immutable nodes configured from the
// NodeGroup I wrote, so that I manage them through the same object as every
// other group and never write per-node configuration by hand.
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

			// The OS image travels as a bare digest: the node compares it with the
			// one it recorded at install to decide a rootfs update, and a tag —
			// which moves under the node — makes that comparison meaningless.
			g.Expect(nc.Spec.OSImage.Digest).To(Equal(testOSImageDigest))

			// Kubelet settings come straight from the NodeGroup.
			g.Expect(nc.Spec.Kubelet.MaxPods).To(Equal(150))
			g.Expect(nc.Spec.Kubelet.ContainerLogMaxSize).To(Equal("100Mi"))
			g.Expect(nc.Spec.Kubelet.ContainerLogMaxFiles).To(Equal(7))
			g.Expect(nc.Spec.Kubelet.ClusterDNS).To(ConsistOf(clusterDNSAddress))
			g.Expect(nc.Spec.Kubelet.ClusterDomain).To(Equal("cluster.local"))
			g.Expect(nc.Spec.Kubelet.NodeLabels).To(HaveKeyWithValue(nodecommon.NodeGroupLabel, internalv1alpha1.NodeLabelValue(ngName)))
			g.Expect(nc.Spec.Kubelet.NodeLabels).To(HaveKeyWithValue("role", internalv1alpha1.NodeLabelValue("worker")))

			// The cluster reads a node's cgroup layout off this label alone;
			// without it node-manager counts the node as unable to run containerd
			// v2 and alerts on it, for every immutable node there is.
			g.Expect(nc.Spec.Kubelet.NodeLabels).To(HaveKeyWithValue(cgroupLabel, internalv1alpha1.NodeLabelValue(cgroupV2Value)))

			// kubelet loads the CA from a file on tmpfs, so the node rewrites
			// it from here on every boot. A config without it leaves a rebooted
			// node unable to start kubelet at all.
			g.Expect(nc.Spec.Kubelet.CACert).To(Equal(base64.StdEncoding.EncodeToString([]byte(testClusterCA))))

			// A NodeGroup has no disk field, but rendering no selector is a dead
			// node ("neither device nor diskSelector set"); the size bound keeps
			// the megabyte cloud-init drive out.
			g.Expect(nc.Spec.Storage).To(Equal(internalv1alpha1.Storage{
				Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: systemDiskSelectorSize}},
			}))

			// The node talks to the API servers the cluster actually has.
			g.Expect(nc.Spec.APIServerEndpoints).To(ConsistOf(apiServerEndpoints))

			// Every immutable node runs these four system extensions, pinned
			// by the digests of this release. The agent is one of them: it is
			// delivered the same way as the rest, so it updates without a reboot.
			g.Expect(nc.Spec.Extensions).To(HaveLen(4))
			byName := map[string]string{}
			for _, ext := range nc.Spec.Extensions {
				byName[ext.Name] = ext.Digest
			}
			g.Expect(byName).To(HaveKeyWithValue(containerdExtension, testContainerdDigest))
			g.Expect(byName).To(HaveKeyWithValue(cniExtension, testCNIDigest))
			g.Expect(byName).To(HaveKeyWithValue(kubeletExtension, testKubeletDigest))
			g.Expect(byName).To(HaveKeyWithValue(nodeletExtension, testNodeletDigest))

			// The update window is the one the operator configured.
			g.Expect(nc.Spec.UpdatePolicy.Window.From).To(Equal("03:00"))
			g.Expect(nc.Spec.UpdatePolicy.Window.To).To(Equal("06:00"))

			g.Expect(nc.Spec.RegistryPackagesProxyAccessTokenB64).NotTo(BeEmpty())

			// This config replaces the bootstrap one wholesale: kubelet does not
			// start without kernel.panic, and the OS renders its hostname from
			// this spec on every boot.
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

	// User story: As a cluster operator, I want the extension I asked for to reach
	// my nodes when I write the request and again when I change it, so that a
	// rebuilt image is one edit away.
	It("re-renders the nodes of a group when a NodeExtensionRequest changes", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-imm")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.Extensions).To(HaveLen(4))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("asking for an extension on the group")
		ner := &v1alpha1.NodeExtensionRequest{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("ext")},
			Spec: v1alpha1.NodeExtensionRequestSpec{
				Sysext:            v1alpha1.Sysext{Name: testRequestedSysextName, Digest: testRequestedSysextDigest},
				NodeGroupSelector: v1alpha1.NodeGroupSelector{MatchNames: []string{ngName}},
			},
		}
		Expect(k8sClient.Create(ctx, ner)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ner) })

		Eventually(func(g Gomega) {
			g.Expect(extensionDigests(getNodeConfig(ctx, g, nodeName))).
				To(HaveKeyWithValue(testRequestedSysextName, testRequestedSysextDigest))

			// The same pass reports the resolution back on the request.
			fresh := &v1alpha1.NodeExtensionRequest{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ner.Name}, fresh)).To(Succeed())
			g.Expect(fresh.Status.MatchedNodeGroups).To(ConsistOf(ngName))
			g.Expect(meta.IsStatusConditionTrue(fresh.Status.Conditions, readyConditionType)).To(BeTrue())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("pointing the request at a rebuilt image")
		patch := client.MergeFrom(ner.DeepCopy())
		ner.Spec.Sysext.Digest = testRequestedSysextRebuiltDigest
		Expect(k8sClient.Patch(ctx, ner, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(extensionDigests(getNodeConfig(ctx, g, nodeName))).
				To(HaveKeyWithValue(testRequestedSysextName, testRequestedSysextRebuiltDigest))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want to be told when a setting I wrote
	// is not the setting my nodes get, so that a group running something I did
	// not configure is visible instead of silent.
	It("says on the NodeGroup which settings it had to clamp", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-clamped")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			// Above anything the pod subnet can ask for, so the API server has to
			// take the ceiling itself; RollingUpdate has no counterpart on a node
			// that is replaced rather than updated in place.
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](1500)}
			ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{ApprovalMode: deckhousev1.DisruptionApprovalModeRollingUpdate}
		})
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			// Clamped to the ceiling, which the CRD takes and which is the top of
			// the bashible ladder: a lower one had immutable nodes advertise half
			// the pods of the bashible nodes beside them on a /21 cluster.
			g.Expect(nc.Spec.Kubelet.MaxPods).To(Equal(1000))
			g.Expect(nc.Spec.UpdatePolicy.Mode).To(Equal(string(deckhousev1.DisruptionApprovalModeAutomatic)))

			messages := clampWarnings(ctx, g, ngName)
			g.Expect(messages).To(ContainElement(ContainSubstring("maxPods")))
			g.Expect(messages).To(ContainElement(ContainSubstring("approvalMode")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want my immutable nodes to advertise
	// the same pod capacity as the bashible nodes beside them, so that the
	// scheduler is not told two different things about one cluster.
	It("takes the pod ceiling from the pod subnet, as bashible does", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-maxpods")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		// The suite's cluster gives each node a /23 of the pod subnet, which is
		// 250 pods for a bashible node — so it is 250 here too, not the flat 120
		// of a NodeConfig with nothing configured.
		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.Kubelet.MaxPods).To(Equal(250))
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

	// One NodeGroup edit drifts every node at once inside a single pass; the
	// group is listed once, so only the pass counting what it already handed
	// out keeps the nodes after the first from passing the same gate.
	It("holds the group to one node when a single pass drifts them all", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-onepass")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](110)}
		})

		nodes := make([]string, 0, 3)
		for range 3 {
			name := testenv.UniqueName("node")
			createNode(ctx, name, ngName)
			nodes = append(nodes, name)
		}

		Eventually(func(g Gomega) {
			for _, name := range nodes {
				g.Expect(getNodeConfig(ctx, g, name).Spec.Kubelet.MaxPods).To(Equal(110))
			}
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		for _, name := range nodes {
			reportApplied(ctx, name)
		}

		By("raising maxPods on the NodeGroup, which drifts all three at once")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(BeNumerically(">=", 1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(Equal(1))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the node that took it reporting back")
		for _, name := range nodes {
			if getNodeConfig(ctx, Default, name).Spec.Kubelet.MaxPods == 200 {
				reportApplied(ctx, name)
			}
		}

		Eventually(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(Equal(2))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As an operator whose immutable nodes have gone quiet, I want a
	// NodeGroup edit to reach them one at a time rather than all together, so a
	// fleet with down agents does not take a change together and interrupt itself
	// together later. Silence on a spec nobody publishes any more does not hold a
	// slot — the group would never get the edit that fixes it — but the node that
	// takes the edit does, so the gate never opens wider than one node.
	It("hands a silent group its change one node at a time", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-silent")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](110)}
		})

		nodes := make([]string, 0, 3)
		for range 3 {
			name := testenv.UniqueName("node")
			createNode(ctx, name, ngName)
			nodes = append(nodes, name)
		}

		// Configured on arrival and then silent: no agent ever reports the spec
		// applied, which is what an absent, outdated or broken agent looks like
		// from here.
		Eventually(func(g Gomega) {
			for _, name := range nodes {
				g.Expect(getNodeConfig(ctx, g, name).Spec.Kubelet.MaxPods).To(Equal(110))
			}
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("raising maxPods on the NodeGroup")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		// One update allowed: the node that takes it holds the only slot, however
		// silent the other two are.
		Eventually(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(Equal(1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(Equal(1))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the node that took it reporting the config it was given")
		for _, name := range nodes {
			if getNodeConfig(ctx, Default, name).Spec.Kubelet.MaxPods == 200 {
				reportApplied(ctx, name)
			}
		}

		// The next node takes it and holds the slot in its turn: the change walks
		// the group, one proof at a time, instead of arriving everywhere at once.
		Eventually(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(Equal(2))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(updatedNodes(ctx, g, nodes...)).To(Equal(2))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: as an operator whose whole group broke on a bad release, I want
	// the fix to reach it. Measured twice on zykov-ab-57a656c4: two workers on
	// generation 1 while the master was on 5, and only raising maxConcurrent moved
	// them.
	It("delivers a new spec to a group where no node has converged", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-wedged")
		createImmutableNodeGroup(ctx, ngName, nil)

		first := testenv.UniqueName("node")
		second := testenv.UniqueName("node")
		createNode(ctx, first, ngName)
		createNode(ctx, second, ngName)

		// Both nodes report the published spec as not applied — a bad release, a
		// registry outage, anything that lands on the whole group at once.
		for _, name := range []string{first, second} {
			Eventually(func(g Gomega) {
				nc := getNodeConfig(ctx, g, name)
				meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
					Type: configurationAppliedCondition, Status: metav1.ConditionFalse,
					Reason: "RollbackFailed", Message: "the previous configuration could not be restored",
				})
				nc.Status.AppliedGeneration = 0
				g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		}

		By("the release publishing the cure")
		setContainerdDigest(ctx, testContainerdRebuiltDigest)

		curedNodes := func(g Gomega) int {
			cured := 0
			for _, name := range []string{first, second} {
				if digestOf(getNodeConfig(ctx, g, name), containerdExtension) == testContainerdRebuiltDigest {
					cured++
				}
			}
			return cured
		}

		Eventually(func(g Gomega) {
			g.Expect(curedNodes(g)).To(Equal(1),
				"exactly one node of a wedged group must get the cure: none means the group is locked, both means the gate is gone")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		// The cure is still a change under test: the node that took it holds the
		// slot until it proves it, so the second node waits its turn.
		Consistently(func(g Gomega) {
			g.Expect(curedNodes(g)).To(Equal(1))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// bashible gives the approval to unhealthy nodes first: fixing what is broken
	// beats touching what works. Ported here for the same reason. The healthy node
	// is created first, so without the ordering it takes the only slot, never
	// reports back, and the broken one waits for a change that never arrives.
	It("gives the free slot to the unhealthy node first", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-priority")
		createImmutableNodeGroup(ctx, ngName, nil)

		healthy := testenv.UniqueName("node")
		broken := testenv.UniqueName("node")
		createNode(ctx, healthy, ngName)
		createNode(ctx, broken, ngName)
		heartbeat(ctx, healthy)

		Eventually(func(g Gomega) {
			node := &corev1.Node{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: broken}, node)).To(Succeed())
			patch := client.MergeFrom(node.DeepCopy())
			node.Status.Conditions = []corev1.NodeCondition{{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionFalse,
				Reason:            "KubeletNotReady",
				Message:           "kubelet stopped posting node status",
				LastHeartbeatTime: metav1.Now(),
			}}
			g.Expect(k8sClient.Status().Patch(ctx, node, patch)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// Both configured and settled first: a node that has just joined is given
		// its config without passing the gate at all, so publishing the change
		// before that has happened would test nothing.
		Eventually(func(g Gomega) {
			g.Expect(digestOf(getNodeConfig(ctx, g, healthy), containerdExtension)).To(Equal(testContainerdDigest))
			g.Expect(digestOf(getNodeConfig(ctx, g, broken), containerdExtension)).To(Equal(testContainerdDigest))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the release publishing a change")
		setContainerdDigest(ctx, testContainerdRebuiltDigest)

		Eventually(func(g Gomega) {
			g.Expect(digestOf(getNodeConfig(ctx, g, broken), containerdExtension)).
				To(Equal(testContainerdRebuiltDigest), "the unhealthy node must be served first")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		// The broken node never reports back, so it holds the only slot: the
		// healthy one is left alone, which is the other half of serving the
		// broken one first.
		Consistently(func(g Gomega) {
			g.Expect(digestOf(getNodeConfig(ctx, g, healthy), containerdExtension)).To(Equal(testContainerdDigest))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As an operator, I want a release that ships a rebuilt system
	// extension to reach the fleet on its own. The digests ConfigMap used to be
	// read but not watched, so the new digest sat unnoticed until some unrelated
	// object moved — in a running cluster that is "whenever", since no resync
	// period is set.
	It("re-renders on a new release digest with nothing else moving", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-digest")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			g.Expect(digestOf(getNodeConfig(ctx, g, nodeName), containerdExtension)).To(Equal(testContainerdDigest))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the release publishing a rebuilt containerd and nothing else happening")
		setContainerdDigest(ctx, testContainerdRebuiltDigest)

		Eventually(func(g Gomega) {
			g.Expect(digestOf(getNodeConfig(ctx, g, nodeName), containerdExtension)).To(Equal(testContainerdRebuiltDigest))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As an operator watching a rootfs update, I want to see how far
	// the node has got. The agent publishes that in status.osImage; a field the
	// CRD does not declare is not merely pruned — the node's whole status apply
	// is refused with "field not declared in schema", so it then reports nothing
	// at all. Measured on zykov-ab-57a656c4 (13.08.2026).
	It("keeps the rootfs update progress the node publishes", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-osimage-status")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			nc.Status.OSImage = &internalv1alpha1.OSImageStatus{
				Digest:       testOSImageDigest,
				Slot:         "a",
				TrialDigest:  testContainerdRebuiltDigest,
				AttemptsLeft: 3,
				FailedDigest: testPauseDigest,
			}
			g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			g.Expect(nc.Status.OSImage).NotTo(BeNil())
			g.Expect(*nc.Status.OSImage).To(Equal(internalv1alpha1.OSImageStatus{
				Digest:       testOSImageDigest,
				Slot:         "a",
				TrialDigest:  testContainerdRebuiltDigest,
				AttemptsLeft: 3,
				FailedDigest: testPauseDigest,
			}))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As an operator with a large fleet, I want heartbeats to trigger
	// nothing. The old detection trick — moving an input nothing watches — is
	// gone with the watch above, so this asserts the contract itself: a heartbeat
	// leaves the rendered config, and its generation, untouched.
	It("does not re-render a node that only reported it is alive", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-heartbeat")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		var generation int64
		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			g.Expect(digestOf(nc, containerdExtension)).To(Equal(testContainerdDigest))
			generation = nc.Generation
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// Settled first: publishing a config is itself an event that brings an
		// all-nodes pass along, and this spec is about what happens when nothing
		// is happening.
		Consistently(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Generation).To(Equal(generation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the node reporting it is still alive")
		heartbeat(ctx, nodeName)

		Consistently(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			g.Expect(nc.Generation).To(Equal(generation))
			g.Expect(digestOf(nc, containerdExtension)).To(Equal(testContainerdDigest))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The gate decides from a list and then writes, so the list must be the API
	// server's: judged against a cache that has not caught up, every node sees
	// "nothing updating" and the whole group takes the change at once.
	It("judges the rollout by the group the API server holds, not by a stale cache", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-gate")
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

		// The group as a lagging cache still holds it: every node applied, nothing
		// updating.
		stale := staleGroupSnapshot(ctx, ngName)

		By("raising maxPods, which the controller hands to one of the two")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		var moved, waiting string
		Eventually(func(g Gomega) {
			moved, waiting = "", ""
			for _, name := range []string{first, second} {
				if getNodeConfig(ctx, g, name).Spec.Kubelet.MaxPods == 200 {
					moved = name
					continue
				}
				waiting = name
			}
			g.Expect(moved).NotTo(BeEmpty())
			g.Expect(waiting).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The gate renders against the group as it now stands, so the node that
		// took the change reads as carrying the published spec unproven.
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		gate := &Reconciler{}
		gate.Client = stale
		gate.sources = &sourceReader{Client: stale, Reader: apiReader}
		gate.derived = &derived_status.Service{Client: k8sClient}

		// One node is mid-update and maxConcurrent defaults to one: the other node
		// must wait, however applied the cache believes the group to be.
		// Each check reads the budget afresh — a budget is per pass.
		Eventually(func(g Gomega) {
			g.Expect(rolloutSlotFor(ctx, g, gate, ng, waiting)).To(BeFalse())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node that took it reporting the spec it was given")
		reportApplied(ctx, moved)

		Eventually(func(g Gomega) {
			g.Expect(rolloutSlotFor(ctx, g, gate, ng, waiting)).To(BeTrue())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want a node that must restart kubelet
	// to be drained first and to see that happening, so the workload leaves
	// before the interruption and I can tell what is done to the node and why.
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
			g.Expect(op.Spec.Type).To(Equal(v1alpha1.NodeOperationTypeApproveDisruption))
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
			g.Expect(node.Annotations).To(HaveKeyWithValue(nodecommon.DrainingAnnotation, HavePrefix("node-operation/")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(findOperation(ctx, g, nodeName).Status.Phase).NotTo(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the drain finishing")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			// The answer echoes the request, the way the draining controller
			// does: the annotation names which operation asked, so a finished
			// one cannot erase a live sibling's request.
			node.Annotations[nodecommon.DrainedAnnotation] = node.Annotations[nodecommon.DrainingAnnotation]
			delete(node.Annotations, nodecommon.DrainingAnnotation)
			node.Spec.Unschedulable = true
			g.Expect(k8sClient.Update(ctx, node)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The eviction finishing is what lets the parent hand the node over.
		Eventually(func(g Gomega) {
			g.Expect(findDrainOf(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))

			parent := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, parent)).To(Succeed())
			g.Expect(parent.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node reporting the operation done")
		Eventually(func(g Gomega) {
			done := findOperation(ctx, g, nodeName)
			done.Status.Phase = v1alpha1.NodeOperationPhaseCompleted
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

	// User story: As a cluster operator, I want one interruption per change, so a
	// node is not drained twice — once for a revision it was already moved off
	// and can never report done, and once for the one it actually runs.
	It("does not ask to interrupt a node for a revision it has already left", func(ctx context.Context) {
		// Manual first, so the node can be left asking for an interruption that
		// nothing has answered — the state the same pass then publishes a new
		// revision over.
		ngName := testenv.UniqueName("workers-revision")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.Kubelet = &deckhousev1.KubeletSpec{MaxPods: ptr.To[int32](110)}
			ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{ApprovalMode: deckhousev1.DisruptionApprovalModeManual}
		})
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		var generation int64
		Eventually(func(g Gomega) {
			generation = getNodeConfig(ctx, g, nodeName).Generation
			g.Expect(generation).NotTo(BeZero())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the agent asking to be interrupted for the config it has")
		requestDisruption(ctx, nodeName, generation)

		By("the operator raising maxPods and handing disruptions back to the cluster")
		ng := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		ng.Spec.Kubelet.MaxPods = ptr.To[int32](200)
		ng.Spec.Disruptions.ApprovalMode = deckhousev1.DisruptionApprovalModeAutomatic
		Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())

		var current int64
		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			g.Expect(nc.Spec.Kubelet.MaxPods).To(Equal(200))
			current = nc.Generation
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The node's status still points at the revision it was moved off; an
		// interruption for that one drains the node for a config nobody will
		// report done and holds it cordoned until the operation times out.
		Consistently(func(g Gomega) {
			g.Expect(approvals(ctx, g, nodeName)).To(BeEmpty())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the agent asking again, for the revision it now has")
		requestDisruption(ctx, nodeName, current)

		Eventually(func(g Gomega) {
			ops := approvals(ctx, g, nodeName)
			g.Expect(ops).To(HaveLen(1))
			g.Expect(ops[0].Spec.ConfigGeneration).To(HaveValue(Equal(current)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want a re-IPed node registered under
	// its actual address, not pinned forever. The judgment must read the API
	// server: the manager's cache strips Node.status.addresses.
	It("releases a pinned node address once the node reports another", func(ctx context.Context) {
		ngName := testenv.UniqueName("master-reip")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("master")

		// A control-plane node publishes its own bootstrap config, so the
		// controller renders over that object rather than creating one.
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
				Labels: map[string]string{
					nodecommon.NodeGroupLabel: ngName,
					controlPlaneRoleLabel:     "",
				},
			},
		}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })
		reportNodeAddress(ctx, nodeName, "10.0.0.10")

		bootstrapped := &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			Spec: internalv1alpha1.NodeSpec{
				NodeName: nodeName,
				OSImage:  internalv1alpha1.OSImage{Digest: testOSImageDigest},
				Kubelet:  internalv1alpha1.Kubelet{NodeIP: "10.0.0.10"},
			},
		}
		Expect(k8sClient.Create(ctx, bootstrapped)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, bootstrapped) })

		// While the node still reports that address, the installer's choice
		// stands: nothing in the cluster can reproduce it.
		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.APIServerEndpoints).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.Kubelet.NodeIP).To(Equal("10.0.0.10"))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the node coming back on a different address")
		reportNodeAddress(ctx, nodeName, "10.0.0.77")

		// An address change is not visible as an event (the cache strips
		// Node.status.addresses); it is read on the next all-nodes pass — in a
		// live cluster, the node's own agent republishing its status.
		reportApplied(ctx, nodeName)

		// Released, so kubelet picks the address the node actually has instead
		// of being handed one it lost.
		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.Kubelet.NodeIP).To(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want a control-plane restart to leave
	// the nodes of my immutable groups alone, so that upgrading or rebooting a
	// master does not walk a configuration change through the whole fleet.
	It("keeps the API server endpoints steady while an apiserver pod is not ready", func(ctx context.Context) {
		pod := createAPIServerPod(ctx, apiServerPodIP)
		endpoint := fmt.Sprintf("https://%s:%d", apiServerPodIP, apiserverPort)

		ngName := testenv.UniqueName("workers-endpoints")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		// The pod is not ready and has never been: its address is published all
		// the same, because the alternative is a node spec that changes shape
		// every time an apiserver is restarted.
		var generation int64
		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			g.Expect(nc.Spec.APIServerEndpoints).To(ContainElement(endpoint))
			generation = nc.Generation
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the apiserver pod becoming ready")
		Eventually(func(g Gomega) {
			fresh := &corev1.Pod{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), fresh)).To(Succeed())
			fresh.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			g.Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		// Pods are not watched here, so the endpoints are recomputed on the next
		// pass whatever triggers it; the group is touched to bring that pass
		// forward rather than waiting for an agent heartbeat.
		touchNodeGroup(ctx, ngName)

		// Readiness is not part of the node's desired state, so the pass changed
		// nothing about the node: no new generation, and so no rollout slot and
		// no candidate interruption.
		Consistently(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)
			g.Expect(nc.Spec.APIServerEndpoints).To(ContainElement(endpoint))
			g.Expect(nc.Generation).To(Equal(generation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		// A master really going away: an evicted or deleted mirror pod keeps its
		// address to the end, so waiting for the object to disappear would keep
		// telling the nodes to talk to it.
		By("the apiserver pod being asked to go")
		Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed())
		touchNodeGroup(ctx, ngName)

		Eventually(func(g Gomega) {
			g.Expect(getNodeConfig(ctx, g, nodeName).Spec.APIServerEndpoints).NotTo(ContainElement(endpoint))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As an operator, I want finished operations to disappear on
	// their own, so that the list stays the record of what is happening rather
	// than everything that ever happened.
	It("collects a finished operation once it is old enough", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-gc")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("old")},
			Spec: v1alpha1.NodeOperationSpec{
				Type:     v1alpha1.NodeOperationTypeDrain,
				NodeName: nodeName,
			},
		}
		Expect(k8sClient.Create(ctx, op)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		By("the operation having finished a day ago")
		Eventually(func(g Gomega) {
			fresh := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, fresh)).To(Succeed())
			fresh.Status.Phase = v1alpha1.NodeOperationPhaseCompleted
			finished := metav1.NewTime(time.Now().Add(-25 * time.Hour))
			fresh.Status.FinishedAt = &finished
			g.Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, &v1alpha1.NodeOperation{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the finished operation must be collected")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The node reports a Reboot done by writing the phase itself, which is not
	// the path that records the time; without a finish time the operation is
	// collected on creation time and the operator-facing field stays empty.
	It("records when an operation the node finished finished", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-stamp")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("reported")},
			Spec: v1alpha1.NodeOperationSpec{
				Type:     v1alpha1.NodeOperationTypeDrain,
				NodeName: nodeName,
			},
		}
		Expect(k8sClient.Create(ctx, op)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		By("the node reporting it done, the way the agent does")
		Eventually(func(g Gomega) {
			fresh := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, fresh)).To(Succeed())
			fresh.Status.Phase = v1alpha1.NodeOperationPhaseCompleted
			fresh.Status.FinishedAt = nil
			g.Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			fresh := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, fresh)).To(Succeed())
			g.Expect(fresh.Status.FinishedAt).NotTo(BeNil(), "the finish time must be recorded")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A record of what was done to a node is worth having while it is recent.
	It("keeps a freshly finished operation", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-keep")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("node")
		createNode(ctx, nodeName, ngName)

		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("recent")},
			Spec: v1alpha1.NodeOperationSpec{
				Type:     v1alpha1.NodeOperationTypeDrain,
				NodeName: nodeName,
			},
		}
		Expect(k8sClient.Create(ctx, op)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		Eventually(func(g Gomega) {
			fresh := &v1alpha1.NodeOperation{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, fresh)).To(Succeed())
			fresh.Status.Phase = v1alpha1.NodeOperationPhaseCompleted
			now := metav1.Now()
			fresh.Status.FinishedAt = &now
			g.Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, &v1alpha1.NodeOperation{})).To(Succeed())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node waiting for permission has not applied anything, whatever its
	// status claims; taking the claim at face value would walk the change
	// through the whole group while every node sat waiting.
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

	// User story: As a cluster operator, I want a master joining an existing
	// cluster to keep the payload it booted with, so that scaling the control
	// plane does not silently move its etcd off the disk the installer chose.
	It("does not race a joining master that cannot label itself yet", func(ctx context.Context) {
		ngName := testenv.UniqueName("master-imm")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.NodeType = deckhousev1.NodeTypeCloudPermanent
			ng.Spec.CloudInstances = nil
		})
		nodeName := testenv.UniqueName("master")

		// A joining master registers WITHOUT the control-plane role label
		// (NodeRestriction); the group label is in the payload's kubelet labels
		// from the start, and it is what must hold the controller back here.
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{nodecommon.NodeGroupLabel: ngName},
			},
		}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })

		By("waiting out the window in which the role label does not exist yet")
		Consistently(func(g Gomega) {
			nc := &internalv1alpha1.NodeConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, nc)).NotTo(Succeed())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want a static node I provisioned by
	// hand to keep the addresses and disks I gave it, so that the cluster does
	// not render DHCP over its static network and lose the node at its next boot.
	It("does not race a static node that publishes the payload it booted with", func(ctx context.Context) {
		ngName := testenv.UniqueName("static-imm")
		createImmutableNodeGroup(ctx, ngName, func(ng *deckhousev1.NodeGroup) {
			ng.Spec.NodeType = deckhousev1.NodeTypeStatic
			ng.Spec.CloudInstances = nil
		})
		nodeName := testenv.UniqueName("static")
		createNode(ctx, nodeName, ngName)

		By("waiting for the node to publish the config it booted with")
		Consistently(func(g Gomega) {
			nc := &internalv1alpha1.NodeConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, nc)).NotTo(Succeed())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		// The payload's shape: an address nothing hands out and a disk the
		// machine was built with. Neither is anything the render can reproduce.
		bootstrapped := &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			Spec: internalv1alpha1.NodeSpec{
				NodeName: nodeName,
				OSImage:  internalv1alpha1.OSImage{Digest: testOSImageDigest},
				Network: internalv1alpha1.Network{
					Hostname: nodeName,
					Interfaces: []internalv1alpha1.NetworkInterface{{
						Name:      "ens3",
						Addresses: []string{"192.168.42.11/24"},
						Gateway:   "192.168.42.1",
					}},
				},
				Storage: internalv1alpha1.Storage{
					Disk: internalv1alpha1.Disk{Device: "/dev/vdb"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, bootstrapped)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, bootstrapped) })

		By("letting the controller render over it")
		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)

			// The render did happen: the cluster-wide inputs are in.
			g.Expect(nc.Spec.APIServerEndpoints).To(ConsistOf(apiServerEndpoints))
			g.Expect(nc.Spec.Extensions).To(HaveLen(4))

			// What only the provisioner knew was not dropped.
			g.Expect(nc.Spec.Network.Interfaces).To(HaveLen(1))
			g.Expect(nc.Spec.Network.Interfaces[0].DHCP).To(BeFalse())
			g.Expect(nc.Spec.Network.Interfaces[0].Addresses).To(ConsistOf("192.168.42.11/24"))
			g.Expect(nc.Spec.Storage.Device).To(Equal("/dev/vdb"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// User story: As a cluster operator, I want the first master to keep working
	// after Deckhouse is installed, so that installing the very thing that
	// manages the cluster does not take its control plane down.
	It("keeps what the installer gave the first master", func(ctx context.Context) {
		ngName := testenv.UniqueName("master-imm")
		createImmutableNodeGroup(ctx, ngName, nil)
		nodeName := testenv.UniqueName("master")

		// The control-plane label is set by the node itself as it brings its
		// apiserver up, which is before it ever reports Ready — so it is there
		// from the first moment the controller can see the node.
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
				Labels: map[string]string{
					nodecommon.NodeGroupLabel: ngName,
					controlPlaneRoleLabel:     "",
				},
			},
		}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })

		// The master publishes the payload it was booted with itself; the
		// controller must not race it with an object of its own, which would
		// carry none of these fields and win over the file.
		By("waiting for the node to publish the config it booted with")
		Consistently(func(g Gomega) {
			nc := &internalv1alpha1.NodeConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, nc)).NotTo(Succeed())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		bootstrapped := &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			Spec: internalv1alpha1.NodeSpec{
				NodeName: nodeName,
				Registry: &internalv1alpha1.Registry{Address: "registry.example.com", Path: "/deckhouse/ce"},
				OSImage:  internalv1alpha1.OSImage{Digest: testOSImageDigest},
				// The installer's shape: its disk plus the one etcd lives on. The
				// mounts mark the section as somebody else's — a selector alone
				// is rendered here too, and keeping it would be write-once.
				Storage: internalv1alpha1.Storage{
					Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
					Mounts: []internalv1alpha1.Mount{{
						Name:              "kubernetes-data",
						PartitionSelector: &internalv1alpha1.PartitionSelector{Size: "10Gi", Blank: true},
						BindTo:            "/var/lib/etcd",
						Mode:              "0700",
					}},
				},
				Kubelet: internalv1alpha1.Kubelet{
					ServerTLSBootstrap:  ptr.To(false),
					ResourceReservation: &internalv1alpha1.ResourceReservation{Mode: "Auto"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, bootstrapped)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, bootstrapped) })

		By("letting the controller render over it")
		Eventually(func(g Gomega) {
			nc := getNodeConfig(ctx, g, nodeName)

			// The render did happen: the cluster-wide inputs are in.
			g.Expect(nc.Spec.APIServerEndpoints).To(ConsistOf(apiServerEndpoints))
			g.Expect(nc.Spec.Extensions).To(HaveLen(4))

			// What only the installer knew was not dropped.
			g.Expect(nc.Spec.Kubelet.ResourceReservation).NotTo(BeNil())
			g.Expect(nc.Spec.Storage.DiskSelector).NotTo(BeNil())
			g.Expect(nc.Spec.Storage.DiskSelector.Size).To(Equal(">=30Gi"))
			g.Expect(nc.Spec.Storage.Mounts).To(HaveLen(1), "etcd must keep the disk it lives on")

			// Registry access, on the other hand, is the cluster's answer now,
			// not the payload's: this node is a control-plane one, so it gets
			// the cluster registry with the credentials from its secret.
			g.Expect(nc.Spec.Registry).NotTo(BeNil())
			g.Expect(nc.Spec.Registry.Address).To(Equal(testRegistryAddress))
			g.Expect(nc.Spec.Registry.Auth).To(Equal(testRegistryAuth))

			// serverTLSBootstrap is left to the cluster: keeping the payload's
			// "false" forever left the master with a self-signed kubelet cert
			// carrying no IP — no exec, no logs, no metrics.
			g.Expect(nc.Spec.Kubelet.ServerTLSBootstrap).To(BeNil())

			// The pause image comes from the cluster's own registry; the
			// upstream one is unreachable in a closed network.
			g.Expect(nc.Spec.ContainerRuntime.SandboxImage).To(Equal(testRegistryAddress + testRegistryPath + "@" + testPauseDigest))
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
		// Deliberately claim the held generation is applied, Ready phase and all:
		// the rollout must not take an overstating agent's word while the same
		// status says the node still waits to be interrupted.
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               configurationAppliedCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "ReconcileSucceeded",
			Message:            "the node is running the published configuration",
			ObservedGeneration: heldGeneration,
		})
		nc.Status.ObservedGeneration = heldGeneration
		nc.Status.AppliedGeneration = heldGeneration
		nc.Status.Phase = phaseReady
		g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// approvals returns every disruption approval that exists for a node, oldest
// first, so a spec can tell "asked once" from "asked again".
func approvals(ctx context.Context, g Gomega, nodeName string) []v1alpha1.NodeOperation {
	ops := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, ops)).To(Succeed())
	var found []v1alpha1.NodeOperation
	for i := range ops.Items {
		if ops.Items[i].Spec.Type == v1alpha1.NodeOperationTypeApproveDisruption && ops.Items[i].Spec.NodeName == nodeName {
			found = append(found, ops.Items[i])
		}
	}
	sort.Slice(found, func(i, j int) bool {
		return found[i].CreationTimestamp.Before(&found[j].CreationTimestamp)
	})
	return found
}

// clampWarnings returns the messages of the warnings the controller published on
// a NodeGroup about settings it could not carry over as written.
func clampWarnings(ctx context.Context, g Gomega, ngName string) []string {
	events := &corev1.EventList{}
	g.Expect(k8sClient.List(ctx, events)).To(Succeed())
	var messages []string
	for i := range events.Items {
		event := &events.Items[i]
		if event.Reason == settingClampedEvent && event.InvolvedObject.Name == ngName {
			messages = append(messages, event.Message)
		}
	}
	return messages
}

// touchNodeGroup annotates the group so the controller re-renders its nodes,
// standing in for whichever event would have brought the next pass along.
func touchNodeGroup(ctx context.Context, ngName string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		ng := &deckhousev1.NodeGroup{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName}, ng)).To(Succeed())
		patch := client.MergeFrom(ng.DeepCopy())
		if ng.Annotations == nil {
			ng.Annotations = map[string]string{}
		}
		ng.Annotations["nodeconfig.test.deckhouse.io/touched"] = time.Now().Format(time.RFC3339Nano)
		g.Expect(k8sClient.Patch(ctx, ng, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// heartbeat is a kubelet saying nothing except that it is still there.
func heartbeat(ctx context.Context, nodeName string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		node := &corev1.Node{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
		node.Status.Conditions = []corev1.NodeCondition{{
			Type:              corev1.NodeReady,
			Status:            corev1.ConditionTrue,
			Reason:            "KubeletReady",
			LastHeartbeatTime: metav1.Now(),
		}}
		g.Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// setContainerdDigest republishes the release's containerd sysext, the way a
// Deckhouse update does. Nothing watches the ConfigMap it lives in, so only a
// render pass picks it up — which is what makes it a probe for whether one ran.
func setContainerdDigest(ctx context.Context, digest string) {
	GinkgoHelper()

	original := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q,"nodeletSysext":%q},"nodeManager":{"olcedar":%q},"common":{"pause":%q}}`,
		testContainerdDigest, testCNIDigest, testKubeletDigest, testNodeletDigest, testOSImageDigest, testPauseDigest)
	updated := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q,"nodeletSysext":%q},"nodeManager":{"olcedar":%q},"common":{"pause":%q}}`,
		digest, testCNIDigest, testKubeletDigest, testNodeletDigest, testOSImageDigest, testPauseDigest)

	writeDigests := func(ctx context.Context, data string) {
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName}, cm)).To(Succeed())
		patch := client.MergeFrom(cm.DeepCopy())
		cm.Data[imagesDigestsKey] = data
		Expect(k8sClient.Patch(ctx, cm, patch)).To(Succeed())
	}
	writeDigests(ctx, updated)
	DeferCleanup(func(ctx context.Context) { writeDigests(ctx, original) })
}

// digestOf returns the digest a config carries for one extension.
func digestOf(nc *internalv1alpha1.NodeConfig, name string) string {
	for _, ext := range nc.Spec.Extensions {
		if ext.Name == name {
			return ext.Digest
		}
	}
	return ""
}

// reportNodeAddress is kubelet publishing the address the node is reachable at.
func reportNodeAddress(ctx context.Context, nodeName, address string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		node := &corev1.Node{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
		node.Status.Addresses = []corev1.NodeAddress{
			{Type: corev1.NodeHostName, Address: nodeName},
			{Type: corev1.NodeInternalIP, Address: address},
		}
		g.Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// rolloutSlotFor asks the gate whether a node may be handed a new spec, reading
// the group afresh the way the first node of a pass does.
func rolloutSlotFor(ctx context.Context, g Gomega, r *Reconciler, ng *deckhousev1.NodeGroup, nodeName string) bool {
	budget, err := r.rolloutBudget(ctx, ng, newPass())
	g.Expect(err).NotTo(HaveOccurred())
	return budget.hasSlot(nodeName)
}

// staleGroupSnapshot is the group's NodeConfigs as a cache that stopped
// receiving updates would hold them: whatever the API server had at this moment,
// frozen.
func staleGroupSnapshot(ctx context.Context, ngName string) client.Client {
	GinkgoHelper()

	list := &internalv1alpha1.NodeConfigList{}
	Expect(k8sClient.List(ctx, list, client.MatchingLabels{nodecommon.NodeGroupLabel: ngName})).To(Succeed())
	objects := make([]client.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, &list.Items[i])
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// createAPIServerPod adds a never-ready kube-apiserver pod, the shape of a
// restarting master; the finalizer makes it linger in Terminating after delete,
// address still on it, the way a real mirror pod does.
func createAPIServerPod(ctx context.Context, podIP string) *corev1.Pod {
	GinkgoHelper()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  kubeSystemNS,
			Name:       testenv.UniqueName("kube-apiserver"),
			Labels:     map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
			Finalizers: []string{terminatingPodFinalizer},
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers:  []corev1.Container{{Name: "kube-apiserver", Image: "kube-apiserver:test"}},
		},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	DeferCleanup(func(ctx context.Context) {
		// The pod has to be gone, not Terminating, before the next spec runs:
		// nothing runs kubelet here to finish the deletion, and a pod left behind
		// goes on publishing its address into every config rendered after it.
		fresh := &corev1.Pod{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), fresh); err == nil {
			testenv.RemoveFinalizers(ctx, k8sClient, fresh)
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, fresh, client.GracePeriodSeconds(0)))).To(Succeed())
		}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	fresh := &corev1.Pod{}
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), fresh)).To(Succeed())
	fresh.Status.PodIP = podIP
	Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
	return pod
}

// findDrainOf returns the eviction a given operation spawned, located the way
// the controller locates it: by ownership, not by a name anyone could take.
func findDrainOf(ctx context.Context, g Gomega, parent string) *v1alpha1.NodeOperation {
	ops := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, ops)).To(Succeed())
	for i := range ops.Items {
		if ops.Items[i].Spec.Type != v1alpha1.NodeOperationTypeDrain {
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
		if ops.Items[i].Spec.NodeName == nodeName && ops.Items[i].Spec.Type != v1alpha1.NodeOperationTypeDrain {
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

// reportApplied is what the node agent does after reconciling the spec it was
// given: the rollout waits for exactly this before moving on.
func reportApplied(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		nc := getNodeConfig(ctx, g, name)
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               configurationAppliedCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "ReconcileSucceeded",
			Message:            "the node is running the published configuration",
			ObservedGeneration: nc.Generation,
		})
		nc.Status.ObservedGeneration = nc.Generation
		nc.Status.AppliedGeneration = nc.Generation
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

	digests := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q,"nodeletSysext":%q},"nodeManager":{"olcedar":%q},"common":{"pause":%q}}`,
		testContainerdDigest, testCNIDigest, testKubeletDigest, testNodeletDigest, testOSImageDigest, testPauseDigest)
	ensureObject(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName},
		Data:       map[string]string{imagesDigestsKey: digests},
	})

	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: registryPackagesProxyTokenSecret},
		Data:       map[string][]byte{registryPackagesProxyTokenKey: []byte("proxy-token")},
	})

	// The registry the cluster was installed from. Rendering refuses to proceed
	// without it: a node whose config names the upstream pause image runs no
	// pods at all in a closed network.
	ensureObject(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: d8SystemNS}})
	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: d8SystemNS, Name: deckhouseRegistrySecret},
		Data: map[string][]byte{
			registryAddressKey:      []byte(testRegistryAddress),
			registryPathKey:         []byte(testRegistryPath),
			registrySchemeKey:       []byte("https"),
			registryImagesKey:       []byte(testRegistryAddress + testRegistryPath),
			registryDockerConfigKey: []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, testRegistryAddress, testRegistryAuth)),
		},
	})

	// kube-controller-manager publishes this ConfigMap in a real cluster;
	// envtest runs the apiserver alone, so the suite creates it. Rendering
	// refuses to proceed without the CA.
	ensureObject(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterCAConfigMap},
		Data:       map[string]string{clusterCAKey: testClusterCA},
	})

	// The cluster's own Kubernetes version: the group's status carries it only
	// once the group has nodes, so this is where the version comes from.
	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterConfigSecretName},
		Data: map[string][]byte{
			clusterConfigKey: []byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterDomain: cluster.local\n" +
				"podSubnetNodeCIDRPrefix: \"23\"\nkubernetesVersion: \"" + testKubernetesVersion + ".6\"\n"),
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
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}
