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

package nodebootstrap

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

	bootstrapv1alpha1 "github.com/deckhouse/node-controller/api/bootstrap.deckhouse.io/v1alpha1"
	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
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

// TestNodeBootstrapControllerEnvtest runs the envtest-backed integration suite:
// the real bootstrap controller runs inside a manager against a real
// kube-apiserver, so what is asserted is the Secret and status a Machine's
// NodeBootstrapConfig ends up with. Skipped when envtest assets are missing so
// the unit tests stay runnable without `make envtest`.
func TestNodeBootstrapControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeBootstrap Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())
	Expect(internalv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(bootstrapv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1beta2.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping envtest with the NodeGroup, NodeConfig, NodeBootstrapConfig and Machine CRDs")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(
		scheme,
		testenv.CRDPaths(
			testenv.WithNodeGroupCRDFile(),
			testenv.WithNodeConfigCRDFile(),
			testenv.WithNodeBootstrapConfigCRDFile(),
			testenv.WithNodeBootstrapConfigTemplateCRDFile(),
			testenv.WithMachineCRDFile(),
		)...,
	)
	Expect(err).NotTo(HaveOccurred())

	By("starting the manager with the node-bootstrap controller")
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
