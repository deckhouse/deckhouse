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

package bootstrapsecrets

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration

	testClusterUUID        = "deadbeef-dead-beef-dead-beefdeadbeef"
	testPackagesProxyToken = "packages-proxy-token-fixture"

	clusterUUIDConfigMapName = "d8-cluster-uuid"
	clusterUUIDKey           = "cluster-uuid"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	suiteCtx  context.Context
	cancel    context.CancelFunc
)

// The controller builds its own bashiblecontext.Service, which reads the cluster CA
// from the pod's projected service-account volume — a path no test process has, and
// BuildInput refuses an empty CA.
func init() {
	bashiblecontext.RootCAFiles = []string{filepath.Join("testdata", "ca.crt")}
}

// TestBootstrapSecrets runs the envtest-backed integration suite for the
// bootstrap-secrets controller against the production cache scoping.
func TestBootstrapSecrets(t *testing.T) {
	if testenv.BinaryAssetsDir() == "" {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrap Secrets Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, cancel = context.WithCancel(context.Background())

	scheme := k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup CRD")
	var err error
	testEnv, _, k8sClient, err = testenv.Start(scheme, testenv.CRDPaths(testenv.WithNodeGroupCRDFile())...)
	Expect(err).NotTo(HaveOccurred())

	By("creating the namespace the bootstrap secrets are written to")
	ns := &corev1.Namespace{}
	ns.Name = nodecommon.MachineNamespace
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ns))).To(Succeed())

	By("publishing the cluster inputs the bootstrap render reads")
	// No EndpointSlice fixture: envtest's own apiserver already registers itself
	// as default/kubernetes, which is where ReadEndpoints finds the masters.
	createClusterUUID()
	create(clusterConfigurationSecret())
	create(clusterKubernetesConfigMap())
	create(imagesDigestsConfigMap())
	create(packagesProxyTokenSecret())
	create(testenv.BootstrapTemplatesConfigMap())

	By("starting the manager with the bootstrap-secrets controller")
	mgr, err := testenv.NewManager(suiteCtx, testEnv.Config, scheme)
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

func create(obj client.Object) {
	GinkgoHelper()
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, obj))).To(Succeed())
}

// createClusterUUID publishes the d8-cluster-uuid ConfigMap. It is a function
// rather than a fixture literal because the empty-UUID spec deletes the object
// and has to put it back for the rest of the suite.
func createClusterUUID() {
	GinkgoHelper()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: clusterUUIDConfigMapName},
		Data:       map[string]string{clusterUUIDKey: testClusterUUID},
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, cm))).To(Succeed())
}

func clusterConfigurationSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: "d8-cluster-configuration"},
		Data: map[string][]byte{"cluster-configuration.yaml": []byte(
			"kubernetesVersion: \"1.31\"\ndefaultCRI: Containerd\nclusterDomain: cluster.local\n" +
				"podSubnetCIDR: 10.111.0.0/16\nserviceSubnetCIDR: 10.222.0.0/16\npodSubnetNodeCIDRPrefix: \"24\"\n")},
	}
}

func clusterKubernetesConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: "d8-cluster-kubernetes"},
		Data:       map[string]string{"spec": "desiredVersion: \"1.31\"\nupdateMode: Manual\n"},
	}
}

// packagesProxyTokenSecret is where the packages-proxy token in the bootstrap
// input comes from. No rendered script in this suite shows it, so without the
// fixture a reader returning nothing would go unnoticed.
//
// The app label is not decoration: the namespace-scoped Secret informer selects on
// it (common/cache.go), and the chart sets it for that reason
// (039-registry-packages-proxy/templates/token.yaml). Drop it and the controller's
// watch on this Secret never fires.
func packagesProxyTokenSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nodecommon.MachineNamespace,
			Name:      bashiblecontext.PackagesProxyTokenSecretName,
			Labels:    map[string]string{"app": "registry-packages-proxy"},
		},
		Data: map[string][]byte{"token": []byte(testPackagesProxyToken)},
	}
}

// imagesDigestsConfigMap carries the digests the bashible templates read by name
// (bb-rpp-get-install reads registrypackages.rppGet, the prerequisites step the rest).
func imagesDigestsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: imagesDigestsConfigMapName},
		Data: map[string]string{imagesDigestsKey: `{"registrypackages":{"jq171":"sha256:jq","d8Curl891":"sha256:curl",` +
			`"tailLog":"sha256:tail","rppGet":"sha256:rpp"}}`},
	}
}
