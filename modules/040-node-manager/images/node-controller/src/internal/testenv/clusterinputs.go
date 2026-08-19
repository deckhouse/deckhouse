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

package testenv

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// The values EnsureClusterInputs publishes. A spec asserting on a rendered node
// config compares against these, so they are the fixture's contract.
const (
	TestKubernetesVersion = "1.35"
	TestContainerdDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	// TestContainerdRebuiltDigest is what a later release ships instead.
	TestContainerdRebuiltDigest = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	TestCNIDigest               = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	TestKubeletDigest           = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	TestPauseDigest             = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	TestNodeletDigest           = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	TestOSImageDigest           = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	TestRegistryAddress         = "registry.example.com"
	TestRegistryPath            = "/deckhouse/ce"
	TestRegistryAuth            = "dXNlcjpwYXNzd29yZA=="
	TestClusterCA               = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
)

// The objects a node's configuration is rendered from. Names mirror the
// constants in internal/controller/nodeconfig/constants.go, which cannot be
// imported here — nodeconfig imports this package.
const (
	imagesDigestsConfigMap = "bashible-apiserver-files"
	imagesDigestsKey       = "images_digests.json"
	proxyTokenSecret       = "registry-packages-proxy-token"
	registrySecretNS       = "d8-system"
	registrySecret         = "deckhouse-registry"
	clusterCAConfigMap     = "kube-root-ca.crt"
	clusterConfigSecret    = "d8-cluster-configuration"
	dnsService             = "kube-dns"
)

// EnsureClusterInputs creates the cluster state a node's configuration is
// rendered from and returns what envtest itself assigned: the DNS ClusterIP and
// the published apiserver endpoints, which the rendered config must name.
// Idempotent, so a BeforeEach can call it for every spec.
func EnsureClusterInputs(ctx context.Context, c client.Client) (string, []string) {
	ginkgo.GinkgoHelper()

	EnsureObject(ctx, c, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nodecommon.KubeSystemNamespace}})
	EnsureObject(ctx, c, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nodecommon.MachineNamespace}})
	EnsureObject(ctx, c, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: registrySecretNS}})

	digests := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q,"nodeletSysext":%q},"nodeManager":{"olcedar":%q},"common":{"pause":%q}}`,
		TestContainerdDigest, TestCNIDigest, TestKubeletDigest, TestNodeletDigest, TestOSImageDigest, TestPauseDigest)
	EnsureObject(ctx, c, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: imagesDigestsConfigMap},
		Data:       map[string]string{imagesDigestsKey: digests},
	})

	EnsureObject(ctx, c, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: proxyTokenSecret},
		Data:       map[string][]byte{"token": []byte("proxy-token")},
	})

	// The registry the cluster was installed from. Rendering refuses to proceed
	// without it: a node whose config names the upstream pause image runs no
	// pods at all in a closed network.
	EnsureObject(ctx, c, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: registrySecretNS, Name: registrySecret},
		Data: map[string][]byte{
			"address":           []byte(TestRegistryAddress),
			"path":              []byte(TestRegistryPath),
			"scheme":            []byte("https"),
			"imagesRegistry":    []byte(TestRegistryAddress + TestRegistryPath),
			".dockerconfigjson": []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, TestRegistryAddress, TestRegistryAuth)),
		},
	})

	// kube-controller-manager publishes this ConfigMap in a real cluster;
	// envtest runs the apiserver alone, so the fixture creates it. Rendering
	// refuses to proceed without the CA.
	EnsureObject(ctx, c, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: clusterCAConfigMap},
		Data:       map[string]string{"ca.crt": TestClusterCA},
	})

	// The cluster's own Kubernetes version: a group's status carries it only
	// once the group has nodes, so this is where the version comes from.
	EnsureObject(ctx, c, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: clusterConfigSecret},
		Data: map[string][]byte{
			"cluster-configuration.yaml": []byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterDomain: cluster.local\n" +
				"podSubnetNodeCIDRPrefix: \"23\"\nkubernetesVersion: \"" + TestKubernetesVersion + ".6\"\n"),
		},
	})

	return ensureDNSService(ctx, c), publishedAPIServerEndpoints(ctx, c)
}

// ensureDNSService creates the DNS service and returns the ClusterIP the envtest
// apiserver allocated for it from its own service CIDR.
func ensureDNSService(ctx context.Context, c client.Client) string {
	ginkgo.GinkgoHelper()

	EnsureObject(ctx, c, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nodecommon.KubeSystemNamespace,
			Name:      dnsService,
			Labels:    map[string]string{"k8s-app": dnsService},
		},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 53, Protocol: corev1.ProtocolUDP}}},
	})

	fresh := &corev1.Service{}
	gomega.Expect(c.Get(ctx, types.NamespacedName{Namespace: nodecommon.KubeSystemNamespace, Name: dnsService}, fresh)).To(gomega.Succeed())
	return fresh.Spec.ClusterIP
}

// publishedAPIServerEndpoints reads the addresses envtest publishes for its own
// apiserver in the default/kubernetes EndpointSlice; the rendered config must
// point the node at exactly those.
func publishedAPIServerEndpoints(ctx context.Context, c client.Client) []string {
	ginkgo.GinkgoHelper()

	slice := &discoveryv1.EndpointSlice{}
	gomega.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, slice)).To(gomega.Succeed())

	var endpoints []string
	for _, endpoint := range slice.Endpoints {
		for _, addr := range endpoint.Addresses {
			for _, port := range slice.Ports {
				if port.Name != nil && *port.Name == "https" && port.Port != nil {
					endpoints = append(endpoints, fmt.Sprintf("https://%s:%d", addr, *port.Port))
				}
			}
		}
	}
	gomega.Expect(endpoints).NotTo(gomega.BeEmpty(), "envtest should publish its apiserver endpoint")
	return endpoints
}

// CreateImmutableNodeGroup creates a CloudEphemeral immutable NodeGroup and
// fills in the Kubernetes version the nodegroup-status controller would publish
// — the kubelet system extension is chosen by it. mutate, if given, shapes the
// group before it is created.
func CreateImmutableNodeGroup(ctx context.Context, c client.Client, name string, mutate ...func(*deckhousev1.NodeGroup)) *deckhousev1.NodeGroup {
	ginkgo.GinkgoHelper()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:   deckhousev1.NodeTypeCloudEphemeral,
			SystemType: deckhousev1.SystemTypeImmutable,
			// A CloudEphemeral group must declare how its nodes are ordered.
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone:     1,
				MaxPerZone:     3,
				ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
			},
		},
	}
	for _, m := range mutate {
		m(ng)
	}
	gomega.Expect(c.Create(ctx, ng)).To(gomega.Succeed())
	ginkgo.DeferCleanup(func(ctx context.Context) { _ = c.Delete(ctx, ng) })

	fresh := &deckhousev1.NodeGroup{}
	gomega.Expect(c.Get(ctx, types.NamespacedName{Name: name}, fresh)).To(gomega.Succeed())
	fresh.Status.KubernetesVersion = TestKubernetesVersion
	gomega.Expect(c.Status().Update(ctx, fresh)).To(gomega.Succeed())
	return fresh
}

// EnsureObject creates the object unless the suite already created it.
func EnsureObject(ctx context.Context, c client.Client, obj client.Object) {
	ginkgo.GinkgoHelper()

	err := c.Create(ctx, obj)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
}
