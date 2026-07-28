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

package webhook

import (
	"context"
	"slices"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/deckhouse/node-controller/internal/testenv"
)

var (
	suiteCtx    context.Context
	suiteCancel context.CancelFunc
	testEnv     *envtest.Environment
	cfg         *rest.Config
	k8sClient   client.Client
	apiWarnings *warningRecorder
)

func TestWebhookControllerEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` or set KUBEBUILDER_ASSETS")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission Webhook Envtest Suite")
}

var _ = BeforeSuite(func() {
	testenv.SetupLogger(GinkgoWriter)
	suiteCtx, suiteCancel = context.WithCancel(context.Background()) // BeforeSuite: t.Context() unavailable

	scheme := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())

	crds := slices.Concat(
		testenv.CRDPaths(testenv.WithNodeManager(testenv.NodeUserCRDFile, testenv.StaticInstanceCRDFile)),
		testenv.ModuleCRDPaths("030-cloud-provider-yandex/candi/openapi/instance_class.yaml"),
	)
	var err error
	testEnv, cfg, k8sClient, err = testenv.StartWithWebhooks(scheme, validatingWebhookConfigurations(), crds...)
	Expect(err).NotTo(HaveOccurred())

	// Rebuild the suite client on a config that records admission warnings, so specs can
	// assert on warnings the same way kubectl surfaces them.
	apiWarnings = &warningRecorder{}
	warnCfg := rest.CopyConfig(cfg)
	warnCfg.WarningHandler = apiWarnings
	k8sClient, err = client.New(warnCfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := testenv.NewManagerWithWebhooks(cfg, scheme, &testEnv.WebhookInstallOptions)
	Expect(err).NotTo(HaveOccurred())
	Expect(SetupWithManager(mgr)).To(Succeed())
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(suiteCtx)).To(Succeed())
	}()
	Expect(testenv.WaitForWebhookServer(suiteCtx, &testEnv.WebhookInstallOptions)).To(Succeed())
})

var _ = AfterSuite(func() {
	if suiteCancel != nil {
		suiteCancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})

// validatingWebhookConfigurations mirrors the production rules from
// modules/040-node-manager/templates/node-controller/webhook.yaml for the migrated
// shell webhooks; envtest rewrites clientConfig to the local webhook server.
func validatingWebhookConfigurations() []*admissionregistrationv1.ValidatingWebhookConfiguration {
	failurePolicy := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone

	newWebhook := func(name, path string, operations []admissionregistrationv1.OperationType, apiVersion string, resources ...string) admissionregistrationv1.ValidatingWebhook {
		return admissionregistrationv1.ValidatingWebhook{
			Name:                    name,
			AdmissionReviewVersions: []string{"v1"},
			FailurePolicy:           &failurePolicy,
			SideEffects:             &sideEffects,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				// envtest joins host:port and path with a slash, so no leading slash here.
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "node-controller-webhook",
					Namespace: "d8-cloud-instance-manager",
					Path:      &path,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: operations,
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"deckhouse.io"},
					APIVersions: []string{apiVersion},
					Resources:   resources,
				},
			}},
		}
	}

	createUpdate := []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
	return []*admissionregistrationv1.ValidatingWebhookConfiguration{{
		ObjectMeta: metav1.ObjectMeta{Name: "node-controller-envtest"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			newWebhook("nodeuser.validation.node-controller.deckhouse.io",
				"validate-deckhouse-io-v1-nodeuser", createUpdate, "*", "nodeusers"),
			newWebhook("staticinstance.validation.node-controller.deckhouse.io",
				"validate-deckhouse-io-v1alpha1-staticinstance", createUpdate, "*", "staticinstances"),
			newWebhook("instanceclass.validation.node-controller.deckhouse.io",
				"validate-instanceclass-delete",
				[]admissionregistrationv1.OperationType{admissionregistrationv1.Delete}, "*", "yandexinstanceclasses"),
		},
	}}
}

// warningRecorder collects admission warnings returned via HTTP Warning headers.
type warningRecorder struct {
	mu       sync.Mutex
	warnings []string
}

func (r *warningRecorder) HandleWarningHeader(_ int, _ string, text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnings = append(r.warnings, text)
}

func (r *warningRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnings = nil
}

func (r *warningRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.warnings)
}
