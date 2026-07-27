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

package draining

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

// TestDrainingControllerEnvtest runs the envtest-backed integration suite. The real draining
// controller runs inside a manager against a real kube-apiserver, so the user-observable state
// transitions (cordon, pod eviction, draining->drained annotation flip) are exercised against a
// real API server rather than a fake client. It is the complement to the pure-logic unit tests in
// controller_test.go. The suite is skipped when envtest assets are not available so the unit tests
// stay runnable without `make envtest`.
func TestDrainingControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Draining Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup CRD")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(
		scheme,
		testenv.CRDPaths(
			testenv.WithNodeGroupCRDFile(),
		)...,
	)
	Expect(err).NotTo(HaveOccurred())

	// The draining controller registered itself via its package init(); since only this package is
	// compiled into the test binary, NewManager wires up just the draining controller.
	By("starting the manager with the draining controller")
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
