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

package crdmigration

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/deckhouse/node-controller/internal/common"
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

// TestCRDMigrationControllerEnvtest runs the conversion-webhook specs against a real apiserver
// with the real shipped CRDs, so the CRD schema, the caBundle round-trip through
// spec.conversion and the "already up to date" comparison are exercised faithfully.
//
// The suite deliberately does NOT start a manager: Setup() loads the embedded CAPI CRD manifests
// from the container path /crds, which does not exist outside the image, so wiring this
// controller into a manager is impossible here. The reconciler is therefore driven by calling
// Reconcile directly against the real apiserver, and the state of the world is asserted.
func TestCRDMigrationControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "CRDMigration Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup CRD")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(
		scheme,
		testenv.CRDPaths(testenv.WithNodeGroupCRDFile())...,
	)
	Expect(err).NotTo(HaveOccurred())

	By("creating the machine namespace")
	ns := &corev1.Namespace{}
	ns.Name = common.MachineNamespace
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ns))).To(Succeed())
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
