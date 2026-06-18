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

package staticproviderid

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

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

// TestStaticProviderIDControllerEnvtest runs the envtest-backed integration suite. It is the
// complement to the fast fake-client unit tests in controller_test.go: here the real
// static-provider-id controller runs inside a manager against a real kube-apiserver, so the
// node watch, the providerID merge patch and multi-reconcile convergence are exercised
// faithfully. The suite is skipped when envtest assets are not available so the unit tests
// stay runnable without `make envtest`.
func TestStaticProviderIDControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "StaticProviderID Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())

	// The controller only reads and writes core Node objects, so no CRDs are needed.
	By("bootstrapping the envtest environment")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(scheme, nil)
	Expect(err).NotTo(HaveOccurred())

	// The static-provider-id controller registered itself via its package init(); since only this
	// package is compiled into the test binary, NewManager wires up just this controller.
	By("starting the manager with the static-provider-id controller")
	mgr, err := testenv.NewManager(cfg, scheme)
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

// ENVTEST_DEBUG=1 dumps cluster state (real `kubectl get … -o wide`) after every spec.
var _ = JustAfterEach(func() {
	if testenv.DebugEnabled() {
		testenv.KubectlDumpNodeObjects(GinkgoWriter, testEnv, cfg, CurrentSpecReport().LeafNodeText)
	}
})
