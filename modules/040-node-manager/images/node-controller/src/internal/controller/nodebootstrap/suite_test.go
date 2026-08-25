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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1alpha1 "github.com/deckhouse/node-controller/api/bootstrap.deckhouse.io/v1alpha1"
	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var k8sClient client.Client

// TestNodeBootstrapControllerEnvtest runs the envtest-backed integration
// suite: the real controller against a real kube-apiserver. Skipped when
// envtest assets are missing so unit tests stay runnable without `make envtest`.
func TestNodeBootstrapControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeBootstrap Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping envtest with the NodeGroup, NodeConfig, NodeBootstrapConfig and Machine CRDs")
	suite, stop := testenv.StartSuite(GinkgoWriter,
		[]func(*k8sruntime.Scheme) error{
			deckhousev1.AddToScheme,
			deckhousev1alpha1.AddToScheme,
			internalv1alpha1.AddToScheme,
			bootstrapv1alpha1.AddToScheme,
			capiv1beta2.AddToScheme,
		},
		testenv.CRDPaths(
			testenv.WithNodeGroupCRDFile(),
			testenv.WithNodeManager(testenv.NodeConfigCRDFile),
			testenv.WithNodeManager(testenv.NodeBootstrapConfigCRDFile),
			testenv.WithMachineCRDFile(),
		)...,
	)
	DeferCleanup(stop)

	k8sClient = suite.Client
})
