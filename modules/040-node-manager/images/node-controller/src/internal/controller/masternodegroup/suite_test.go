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

// TestMasterNodeGroupControllerEnvtest runs the envtest-backed integration suite. Most specs drive
// Reconcile directly, because each case needs its own cluster configuration. The specs that need
// the controller's own startup enqueue and watches run under a manager started by startManager.
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
})

// startManager runs the master-node-group controller until the calling container ends. A running
// controller reacts to every change of NodeGroup master, so the specs that drive Reconcile directly
// must not run under it. Call it once: controller-runtime refuses a second controller of one name.
func startManager() {
	ctx, cancel := context.WithCancel(suiteCtx)
	mgr, err := testenv.NewManager(ctx, cfg, scheme)
	Expect(err).NotTo(HaveOccurred())

	stopped := make(chan struct{})
	go func() {
		defer GinkgoRecover()
		defer close(stopped)
		Expect(mgr.Start(ctx)).To(Succeed())
	}()

	DeferCleanup(func() {
		cancel()
		Eventually(stopped, eventuallyTimeout).Should(BeClosed())
	})
}

var _ = AfterSuite(func() {
	By("tearing down the envtest environment")
	if suiteCancel != nil {
		suiteCancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})
