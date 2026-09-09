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

package capi

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	envtestClusterUUID = "f49dd1c3-a63a-4565-a06c-625e35587eab"
	envtestZone        = "zone-a"
)

var (
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
	scheme    *k8sruntime.Scheme

	suiteCtx    context.Context
	suiteCancel context.CancelFunc
)

// TestCAPIMachineDeploymentControllerEnvtest runs the MachineDeployment reconciler inside a
// manager against a real apiserver with the production cache scoping for core types, so the
// ConfigMap watch is exercised end to end. The other capi controllers are disabled: their
// CRDs (Cluster, MachineHealthCheck, DeckhouseControlPlane) are not part of this suite.
func TestCAPIMachineDeploymentControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "CAPI MachineDeployment Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1beta2.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup, MCM and MachineDeployment CRDs")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(
		scheme,
		testenv.CRDPaths(
			testenv.WithNodeGroupCRDFile(),
			testenv.WithMCMCRDFile(),
			testenv.WithMachineDeploymentCRDFile(),
		)...,
	)
	Expect(err).NotTo(HaveOccurred())

	By("creating the machine namespace and the cluster-wide inputs the reconciler reads")
	ns := &corev1.Namespace{}
	ns.Name = common.MachineNamespace
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ns))).To(Succeed())

	Expect(k8sClient.Create(suiteCtx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterUUIDConfigMapName, Namespace: clusterUUIDConfigMapNS},
		Data:       map[string]string{"cluster-uuid": envtestClusterUUID},
	})).To(Succeed())
	Expect(k8sClient.Create(suiteCtx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace},
		StringData: map[string]string{
			"capiClusterName":         "envtest",
			"capiMachineTemplateKind": "DeckhouseMachineTemplate",
			// envtest has no CAPI defaulting webhook, which fills spec.selector in a real cluster.
			"capiMachineDeploymentSpecPatch": "selector:\n  matchLabels:\n    node-group: ${nodeGroupName}\n",
		},
	})).To(Succeed())

	By("starting the manager with only the capi-machine-deployment controller")
	cacheOpts, clientOpts := common.CacheOptions()
	cacheOpts.ByObject = coreTypesOnly(cacheOpts)
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme,
		Cache:          cacheOpts,
		Client:         clientOpts,
		Metrics:        metricsserver.Options{BindAddress: "0"},
		LeaderElection: false,
	})
	Expect(err).NotTo(HaveOccurred())
	disabled := "capi-api-version,capi-cluster-resources,capi-control-plane,capi-finalizer-cleanup,capi-md-metrics"
	Expect(register.SetupAll(mgr, mgr.GetClient(), disabled, 1, nil)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(suiteCtx)).To(Succeed())
	}()
})

// coreTypesOnly keeps the production ByObject scoping for Secrets and ConfigMaps and drops the
// CRD-backed entries: the manager refuses to start when a scoped type has no CRD installed.
func coreTypesOnly(opts cache.Options) map[client.Object]cache.ByObject {
	kept := make(map[client.Object]cache.ByObject)
	for obj, byObject := range opts.ByObject {
		switch obj.(type) {
		case *corev1.Secret, *corev1.ConfigMap:
			kept[obj] = byObject
		}
	}
	return kept
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
