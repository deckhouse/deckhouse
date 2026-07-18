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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
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

	It("leaves nodes of a bashible-managed group alone", func(ctx context.Context) {
		ngName := testenv.UniqueName("workers-mutable")
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: ngName},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				OSType:   deckhousev1.OSTypeMutable,
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
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			OSType:   deckhousev1.OSTypeImmutable,
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
