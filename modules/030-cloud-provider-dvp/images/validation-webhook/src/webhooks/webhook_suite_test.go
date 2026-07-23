// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
)

var (
	testCtx       context.Context
	testCancel    context.CancelFunc
	testEnv       *envtest.Environment
	testConfig    *rest.Config
	testScheme    *runtime.Scheme
	testK8sClient client.Client
)

func TestWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DVP Validation Webhook Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testCtx, testCancel = context.WithCancel(context.Background())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("testdata", "crds"),
		},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("testdata", "webhook")},
		},
	}
	assetsDir := firstFoundEnvTestBinaryDir()
	if assetsDir == "" {
		Skip("envtest assets not found; run `make envtest` or set KUBEBUILDER_ASSETS")
	}
	testEnv.BinaryAssetsDirectory = assetsDir

	var err error
	testConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(testConfig).NotTo(BeNil())

	testScheme = runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(testScheme)).To(Succeed())
	Expect(admissionv1.AddToScheme(testScheme)).To(Succeed())
	Expect(apiextensionsv1.AddToScheme(testScheme)).To(Succeed())
	registerUnstructuredGVKs(testScheme)

	testK8sClient, err = client.New(testConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	webhookOptions := &testEnv.WebhookInstallOptions
	webhookServer := webhook.NewServer(webhook.Options{
		Host:    webhookOptions.LocalServingHost,
		Port:    webhookOptions.LocalServingPort,
		CertDir: webhookOptions.LocalServingCertDir,
	})

	mgr, err := ctrl.NewManager(testConfig, ctrl.Options{
		Scheme: testScheme,
		Metrics: metricsserver.Options{
			BindAddress:   fmt.Sprintf("%s:%d", webhookOptions.LocalServingHost, webhookOptions.LocalServingPort+1),
			SecureServing: false,
		},
		WebhookServer:  webhookServer,
		LeaderElection: false,
	})
	Expect(err).NotTo(HaveOccurred())

	builder := cpvaladmission.NewStateBuilder(mgr.GetClient(), cpvaladmission.StateBuilderConfig{
		ModuleName:        dvpmeta.ModuleName,
		NamespaceName:     dvpmeta.Namespace,
		InstanceClassKind: dvpmeta.InstanceClassKind,
	})
	Expect(NewCredentialSecretValidator(builder, &corev1.Secret{}).Register(mgr)).To(Succeed())
	Expect(NewNodeGroupValidator(builder, newWebhookTestObject(nodeGroupGVK())).Register(mgr)).To(Succeed())
	Expect(NewDVPInstanceClassValidator(builder, newWebhookTestObject(instanceClassGVK())).Register(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(testCtx)).To(Succeed())
	}()

	dialer := &net.Dialer{Timeout: time.Second}
	address := fmt.Sprintf("%s:%d", webhookOptions.LocalServingHost, webhookOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		return conn.Close()
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	testCancel()
	Expect(testEnv.Stop()).To(Succeed())
})

func registerUnstructuredGVKs(scheme *runtime.Scheme) {
	for _, gvk := range []schema.GroupVersionKind{nodeGroupGVK(), instanceClassGVK()} {
		listGVK := gvk
		listGVK.Kind += "List"
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})
	}
}

func newWebhookTestObject(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return obj
}

func nodeGroupGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
}

func instanceClassGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: dvpmeta.InstanceClassKind}
}

func firstFoundEnvTestBinaryDir() string {
	if assetsDir := os.Getenv("KUBEBUILDER_ASSETS"); assetsDir != "" {
		return assetsDir
	}

	for _, basePath := range []string{filepath.Join("bin", "k8s"), filepath.Join("..", "bin", "k8s")} {
		if found := firstDirWithFile(basePath, "etcd"); found != "" {
			return found
		}
	}

	return ""
}

func firstDirWithFile(basePath, fileName string) string {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		path := filepath.Join(basePath, entry.Name())
		if !entry.IsDir() && entry.Name() == fileName {
			return basePath
		}
		if entry.IsDir() {
			if found := firstDirWithFile(path, fileName); found != "" {
				return found
			}
		}
	}

	return ""
}
