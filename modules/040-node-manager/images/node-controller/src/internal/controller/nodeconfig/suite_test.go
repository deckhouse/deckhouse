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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	_ "github.com/deckhouse/node-controller/internal/controller/nodeoperation"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var (
	suite     *testenv.Suite
	k8sClient client.Client

	// apiReader reads straight from the API server, the way the running
	// controller's uncached reader does. Specs that build a reconciler of their
	// own use it so that reconciler reads what the cluster actually holds.
	apiReader client.Reader
)

// TestNodeConfigControllerEnvtest runs the envtest-backed integration suite:
// the real controller against a real kube-apiserver, asserting the NodeConfigs
// the cluster ends up with. Skipped when envtest assets are missing.
func TestNodeConfigControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeConfig Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	// Both controllers registered themselves via their package init(): the
	// disruption of a node is a conversation between them, so the suite runs
	// the pair the cluster runs.
	By("bootstrapping envtest with the NodeGroup, NodeConfig and NodeOperation CRDs")
	var stop func()
	suite, stop = testenv.StartSuite(GinkgoWriter,
		[]func(*k8sruntime.Scheme) error{
			deckhousev1.AddToScheme,
			internalv1alpha1.AddToScheme,
			v1alpha1.AddToScheme,
		},
		testenv.CRDPaths(
			testenv.WithNodeGroupCRDFile(),
			testenv.WithNodeManager(testenv.NodeConfigCRDFile),
			testenv.WithNodeManager(testenv.NodeOperationCRDFile),
		)...,
	)
	DeferCleanup(stop)

	k8sClient = suite.Client
	apiReader = suite.APIReader
})

// ENVTEST_DEBUG=1 dumps cluster state (real `kubectl get … -o wide`) after every spec.
var _ = JustAfterEach(func() {
	if testenv.DebugEnabled() {
		testenv.KubectlDumpNodeObjects(GinkgoWriter, suite.Env, suite.Config, CurrentSpecReport().LeafNodeText)
	}
})
