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

package nodegroup

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
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

// TestNodeGroupControllerEnvtest runs the envtest-backed integration suite for the nodegroup
// status controller. It is the complement to the fast fake-client unit tests in
// controller_test.go: here the real controller runs inside a manager against a real
// kube-apiserver, so the status subresource it writes (counters, conditions, condition summary,
// processed/synced) is asserted as the actual state of the world after the controller reacts to
// NodeGroups, Nodes, Machines and the checksum Secret through its real watches. The suite skips
// when envtest assets are absent so the unit tests stay runnable without `make envtest`.
func TestNodeGroupControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeGroup Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(v1.AddToScheme(scheme)).To(Succeed())
	Expect(mcmv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1beta2.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the nodegroup CRDs")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(scheme, testenv.CRDPaths(
		"node_group.yaml", "mcm.yaml", "machine.yaml", "machine-deployment.yaml"))
	Expect(err).NotTo(HaveOccurred())

	By("creating the machine namespace")
	ns := &corev1.Namespace{}
	ns.Name = common.MachineNamespace
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ns))).To(Succeed())

	// The nodegroup status controller registered itself via its package init(); since only this
	// package is compiled into the test binary, NewManager wires up just this controller.
	By("starting the manager with the nodegroup status controller")
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
