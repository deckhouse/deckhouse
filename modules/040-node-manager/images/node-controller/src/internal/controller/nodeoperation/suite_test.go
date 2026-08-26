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

package nodeoperation

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var k8sClient client.Client

// TestNodeOperationControllerEnvtest runs the envtest-backed integration
// suite: the real controller against a real kube-apiserver. Skipped when
// envtest assets are missing so unit tests stay runnable without `make envtest`.
func TestNodeOperationControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeOperation Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping envtest with the NodeOperation and NodeGroup CRDs")
	suite, stop := testenv.StartSuite(GinkgoWriter,
		[]func(*k8sruntime.Scheme) error{
			deckhousev1alpha1.AddToScheme,
			// NodeGroup: the wait for an eviction is bounded by the group's own
			// nodeDrainTimeoutSecond, so the controller reads the node's group.
			deckhousev1.AddToScheme,
		},
		testenv.CRDPaths(
			testenv.WithNodeManager(testenv.NodeOperationCRDFile),
			testenv.WithNodeGroupCRDFile(),
		)...,
	)
	DeferCleanup(stop)

	k8sClient = suite.Client
})
