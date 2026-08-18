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

package bashiblecontext

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	scheme    *k8sruntime.Scheme
	suiteCtx  context.Context
	cancel    context.CancelFunc
)

// TestBashibleContextControllerEnvtest runs the envtest-backed integration suite for the
// bashible-context controller against the production cache scoping.
func TestBashibleContextControllerEnvtest(t *testing.T) {
	if testenv.BinaryAssetsDir() == "" {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bashible Context Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, cancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup CRD")
	var err error
	testEnv, _, k8sClient, err = testenv.Start(scheme, testenv.CRDPaths(testenv.WithNodeGroupCRDFile())...)
	Expect(err).NotTo(HaveOccurred())
	cfg := testEnv.Config

	By("creating the namespaces the context is assembled from and written to")
	for _, name := range []string{cloudInstanceManagerNS, versionInfoCMNS} {
		ns := &corev1.Namespace{}
		ns.Name = name
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ns))).To(Succeed())
	}

	By("publishing the cluster DNS service")
	// Without a discovered DNS address the controller refuses to publish anything, so this
	// fixture is what makes every other assertion in the suite reachable.
	dns := &corev1.Service{}
	dns.Namespace = kubeSystemNS
	dns.Name = "kube-dns"
	dns.Labels = map[string]string{"k8s-app": "kube-dns"}
	dns.Spec = corev1.ServiceSpec{
		Ports:    []corev1.ServicePort{{Port: 53, Protocol: corev1.ProtocolUDP, Name: "dns"}},
		Selector: map[string]string{"k8s-app": "kube-dns"},
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, dns))).To(Succeed())

	By("publishing the cluster-configuration secret")
	clusterCfg := &corev1.Secret{}
	clusterCfg.Namespace = kubeSystemNS
	clusterCfg.Name = clusterConfigSecretName
	clusterCfg.Data = map[string][]byte{
		clusterConfigKey: []byte("kubernetesVersion: \"1.31\"\ndefaultCRI: Containerd\nclusterDomain: cluster.local\npodSubnetCIDR: 10.111.0.0/16\nserviceSubnetCIDR: 10.222.0.0/16\npodSubnetNodeCIDRPrefix: \"24\"\n"),
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, clusterCfg))).To(Succeed())

	By("publishing the d8-cluster-kubernetes ConfigMap")
	clusterKubernetes := &corev1.ConfigMap{}
	clusterKubernetes.Namespace = kubeSystemNS
	clusterKubernetes.Name = "d8-cluster-kubernetes"
	clusterKubernetes.Data = map[string]string{
		"spec": "desiredVersion: \"1.32\"\nupdateMode: Manual\n",
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, clusterKubernetes))).To(Succeed())

	By("pre-issuing the api-proxy discovery certificate")
	// envtest runs no CSR signer, so letting the controller issue this certificate would block
	// every assembly for the full csrWaitTimeout. A certificate that is not due for renewal
	// makes ensureCertificate a no-op.
	certSecret := &corev1.Secret{}
	certSecret.Namespace = kubeSystemNS
	certSecret.Name = apiProxyCertSecretName
	certSecret.Data = map[string][]byte{"crt": selfSignedCert(), "key": []byte("unused-by-the-controller")}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, certSecret))).To(Succeed())

	By("creating the kubernetes EndpointSlice")
	// envtest runs no endpointslice mirroring controller, so this object is the suite's alone
	// to own — nothing rewrites it behind the specs.
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, newKubernetesEndpointSlice("10.10.0.1")))).To(Succeed())

	By("starting the manager with the bashible-context controller")
	mgr, err := testenv.NewManager(suiteCtx, cfg, scheme)
	Expect(err).NotTo(HaveOccurred())
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(suiteCtx)).To(Succeed())
	}()
	Expect(mgr.GetCache().WaitForCacheSync(suiteCtx)).To(BeTrue())
})

var _ = AfterSuite(func() {
	By("tearing down the envtest environment")
	if cancel != nil {
		cancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})

func newKubernetesEndpointSlice(address string) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubernetesEndpointSliceNS,
			Name:      kubernetesEndpointSliceName,
			Labels:    map[string]string{discoveryv1.LabelServiceName: "kubernetes"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports: []discoveryv1.EndpointPort{{
			Name: ptr.To("https"),
			Port: ptr.To(int32(6443)),
		}},
		Endpoints: []discoveryv1.Endpoint{{Addresses: []string{address}}},
	}
}

func selfSignedCert() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: certCommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
