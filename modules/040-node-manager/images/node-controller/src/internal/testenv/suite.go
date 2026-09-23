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

package testenv

import (
	"context"
	"io"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Suite is a running envtest apiserver with the controllers of the test binary
// wired to it. Ctx is the manager's context: it is cancelled by the cleanup
// StartSuite returns.
type Suite struct {
	Env       *envtest.Environment
	Config    *rest.Config
	Client    client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
	Ctx       context.Context
}

// StartSuite boots envtest with the client-go scheme plus the given builders and
// CRD files, and starts a manager wired with every controller that registered
// itself in the test binary. Call the returned cleanup from the suite teardown
// (DeferCleanup in BeforeSuite does), and skip on !AssetsAvailable() first.
func StartSuite(w io.Writer, addToScheme []func(*runtime.Scheme) error, crdPaths ...string) (*Suite, func()) {
	ginkgo.GinkgoHelper()
	return startSuite(w, addToScheme, nil, nil, crdPaths)
}

// StartSuiteWithWebhooks is StartSuite with the given ValidatingWebhookConfigurations
// installed, so the specs see the admission the cluster sees. setupWebhooks registers
// the handlers on the manager (pass webhook.SetupWithManager); it blocks until the
// webhook server answers.
func StartSuiteWithWebhooks(w io.Writer, addToScheme []func(*runtime.Scheme) error,
	validating []*admissionregistrationv1.ValidatingWebhookConfiguration,
	setupWebhooks func(manager.Manager) error, crdPaths ...string,
) (*Suite, func()) {
	ginkgo.GinkgoHelper()
	return startSuite(w, addToScheme, validating, setupWebhooks, crdPaths)
}

func startSuite(w io.Writer, addToScheme []func(*runtime.Scheme) error,
	validating []*admissionregistrationv1.ValidatingWebhookConfiguration,
	setupWebhooks func(manager.Manager) error, crdPaths []string,
) (*Suite, func()) {
	ginkgo.GinkgoHelper()

	SetupLogger(w)
	ctx, cancel := context.WithCancel(context.Background())

	scheme := runtime.NewScheme()
	gomega.Expect(clientgoscheme.AddToScheme(scheme)).To(gomega.Succeed())
	for _, add := range addToScheme {
		gomega.Expect(add(scheme)).To(gomega.Succeed())
	}

	var (
		env *envtest.Environment
		cfg *rest.Config
		c   client.Client
		err error
	)
	if len(validating) == 0 {
		env, cfg, c, err = Start(scheme, crdPaths...)
	} else {
		env, cfg, c, err = StartWithWebhooks(scheme, validating, crdPaths...)
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	mgr, err := NewManagerWithWebhooks(ctx, cfg, scheme, webhookOptions(env, validating))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	if setupWebhooks != nil {
		gomega.Expect(setupWebhooks(mgr)).To(gomega.Succeed())
	}

	go func() {
		defer ginkgo.GinkgoRecover()
		gomega.Expect(mgr.Start(ctx)).To(gomega.Succeed())
	}()

	if len(validating) > 0 {
		gomega.Expect(WaitForWebhookServer(ctx, &env.WebhookInstallOptions)).To(gomega.Succeed())
	}

	suite := &Suite{
		Env:       env,
		Config:    cfg,
		Client:    c,
		APIReader: mgr.GetAPIReader(),
		Scheme:    scheme,
		Ctx:       ctx,
	}
	return suite, func() {
		cancel()
		gomega.Expect(env.Stop()).To(gomega.Succeed())
	}
}

// webhookOptions is nil unless the suite installed webhooks, so a suite without
// them gets a manager with no webhook server, exactly as before.
func webhookOptions(env *envtest.Environment, validating []*admissionregistrationv1.ValidatingWebhookConfiguration) *envtest.WebhookInstallOptions {
	if len(validating) == 0 {
		return nil
	}
	return &env.WebhookInstallOptions
}
