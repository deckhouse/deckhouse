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
	"path/filepath"
	"runtime"
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
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var (
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
	scheme    *k8sruntime.Scheme
	suiteCtx  context.Context
	cancel    context.CancelFunc
)

// capiMachineTemplateFixture renders a DeckhouseMachineTemplate whose name and node-group
// label come from the same render context production templates use.
const capiMachineTemplateFixture = `apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: DeckhouseMachineTemplate
metadata:
  name: {{ .templateName }}
  namespace: d8-cloud-instance-manager
  labels:
    node-group: {{ .nodeGroup.name }}
spec:
  template:
    spec:
      vmClassName: test
`

// instanceClassChecksumFixture keeps the checksum stable per NodeGroup, matching the
// contract that a changed instance class changes the checksum (irrelevant for this suite).
const instanceClassChecksumFixture = `{{ .nodeGroup.name }}`

func testdataDir() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "testdata")
}

// TestCAPIMachineDeploymentControllerEnvtest runs the envtest-backed integration suite for
// the capi package controllers against the production cache scoping.
func TestCAPIMachineDeploymentControllerEnvtest(t *testing.T) {
	if testenv.BinaryAssetsDir() == "" {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run the integration suite")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "CAPI MachineDeployment Controller Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, cancel = context.WithCancel(context.Background())

	scheme = k8sruntime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(deckhousev1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1beta2.AddToScheme(scheme)).To(Succeed())
	Expect(mcmv1alpha1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping the envtest environment with the NodeGroup and provider CRDs")
	var err error
	testEnv, cfg, k8sClient, err = testenv.Start(
		scheme,
		append(
			testenv.CRDPaths(testenv.WithNodeGroupCRDFile()),
			filepath.Join(testdataDir(), "dvpinstanceclass-crd.yaml"),
			filepath.Join(testdataDir(), "deckhousemachinetemplate-crd.yaml"),
		)...,
	)
	Expect(err).NotTo(HaveOccurred())

	By("creating the machine namespace")
	ns := &corev1.Namespace{}
	ns.Name = common.MachineNamespace
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ns))).To(Succeed())

	By("publishing the cloud-provider discovery secret (CAPI engine, DVP-like)")
	cloudProvider := &corev1.Secret{}
	cloudProvider.Namespace = cloudProviderSecretNamespace
	cloudProvider.Name = cloudProviderSecretName
	cloudProvider.Data = map[string][]byte{
		"type":                          []byte(`"dvp"`),
		"instanceClassKind":             []byte(`"DVPInstanceClass"`),
		"capiClusterKind":               []byte(`"DeckhouseCluster"`),
		"capiClusterName":               []byte("dvp"),
		"capiMachineTemplateKind":       []byte("DeckhouseMachineTemplate"),
		"capiMachineTemplateAPIVersion": []byte("infrastructure.cluster.x-k8s.io/v1alpha1"),
		"zones":                         []byte(`["zone-a"]`),
		// In production the CAPI mutating webhook defaults spec.selector; envtest has no
		// CAPI webhooks, so the provider spec-patch (a production mechanism) supplies a
		// selector matching the node-group template label.
		"capiMachineDeploymentSpecPatch": []byte(`{"selector":{"matchLabels":{"node-group":"{{ .nodeGroupName }}"}}}`),
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, cloudProvider))).To(Succeed())

	By("publishing the cluster-configuration secret")
	clusterCfg := &corev1.Secret{}
	clusterCfg.Namespace = clusterConfigSecretNamespace
	clusterCfg.Name = clusterConfigSecretName
	// The derived-status service and the pod-subnet reader resolve this secret by its
	// production name, so the fixture name must match exactly.
	clusterCfg.Data = map[string][]byte{
		"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.31\"\ndefaultCRI: Containerd\npodSubnetCIDR: 10.111.0.0/16\n"),
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, clusterCfg))).To(Succeed())

	By("publishing the provider CAPI template secret")
	templates := &corev1.Secret{}
	templates.Namespace = providerTemplateSecretNamespace
	templates.Name = "d8-cloud-provider-dvp-capi"
	templates.Data = map[string][]byte{
		"machine-template.yaml":  []byte(capiMachineTemplateFixture),
		"instance-class.checksum": []byte(instanceClassChecksumFixture),
	}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, templates))).To(Succeed())

	By("publishing the cluster-uuid configmap")
	uuidCM := &corev1.ConfigMap{}
	uuidCM.Namespace = clusterUUIDConfigMapNS
	uuidCM.Name = clusterUUIDConfigMapName
	uuidCM.Data = map[string]string{"cluster-uuid": "11111111-2222-3333-4444-555555555555"}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, uuidCM))).To(Succeed())

	By("starting the manager with the capi controllers")
	mgr, err := testenv.NewManager(cfg, scheme)
	Expect(err).NotTo(HaveOccurred())
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(suiteCtx)).To(Succeed())
	}()
	Expect(mgr.GetCache().WaitForCacheSync(suiteCtx)).To(BeTrue())
})

var _ = AfterSuite(func() {
	By("tearing down the envtest environment")
	if cancel != nil {
		cancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})
