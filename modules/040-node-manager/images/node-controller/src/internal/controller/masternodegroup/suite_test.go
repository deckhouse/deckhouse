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
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var (
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
	scheme    *k8sruntime.Scheme

	suiteCtx    context.Context
	suiteCancel context.CancelFunc
)

// TestMasterNodeGroupControllerEnvtest runs the envtest-backed integration suite. The cluster
// configuration Secret is published before the manager starts, so the first spec observes the real
// startup path: the controller's one-shot enqueue creating the master NodeGroup against a real
// apiserver. The remaining specs drive Reconcile directly, because the startup source fires once
// per manager and each case needs its own cluster configuration.
func TestMasterNodeGroupControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "MasterNodeGroup Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup CRD")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(scheme, testenv.CRDPaths(testenv.WithNodeGroupCRDFile())...)
	Expect(err).NotTo(HaveOccurred())

	By("publishing the cluster configuration of a cloud cluster")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName},
		Data:       map[string][]byte{clusterConfigKey: []byte("clusterType: Cloud\n")},
	}
	Expect(k8sClient.Create(suiteCtx, secret)).To(Succeed())

	By("starting the manager with the master-node-group controller")
	mgr, err := testenv.NewManager(suiteCtx, cfg, scheme)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(suiteCtx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the envtest environment")
	if suiteCancel != nil {
		suiteCancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})
