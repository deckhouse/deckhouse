/*
Copyright 2024 Flant JSC

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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	namespacecontroller "controller/internal/controller/namespace"
	projectcontroller "controller/internal/controller/project"
	templatecontroller "controller/internal/controller/template"
	grantcontrollers "controller/internal/controllers"
	clusterprojectrolebindingcontroller "controller/internal/controllers/clusterprojectrolebinding"
	projectrolebindingcontroller "controller/internal/controllers/projectrolebinding"
	"controller/internal/helm"
	"controller/internal/jsonpath"
	clusterprojectrolebindingwebhook "controller/internal/webhook/clusterprojectrolebinding"
	namespacewebhook "controller/internal/webhook/namespace"
	projectwebhook "controller/internal/webhook/project"
	projectrolebindingwebhook "controller/internal/webhook/projectrolebinding"
	templatewebhook "controller/internal/webhook/template"
	grantwebhooks "controller/internal/webhooks"
)

var (
	// path to helm templates
	helmTemplatesPath = "helmlib"
	// path to default project templates
	templatesPath = "templates"
	// helm release namespace
	helmNamespace = "d8-multitenancy-manager"
	// controller service account
	serviceAccount = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	// list of service accounts allowed to create namespaces when allowNamespacesWithoutProjects is set to false
	allowedServiceAccounts = []string{serviceAccount, "system:serviceaccount:d8-system:deckhouse", "system:serviceaccount:d8-upmeter:upmeter-agent"}
)

const (
	haModeEnv      = "HA_MODE"
	controllerName = "multitenancy-manager"
)

func main() {
	var allowOrphanNamespaces bool
	flag.BoolVar(&allowOrphanNamespaces, "allow-orphan-namespaces", true, "allow to create a namespace which is not a part of a Project")
	flag.Parse()

	// setup logger
	logger := ctrl.Log.WithName(controllerName)
	ctrllog.SetLogger(zap.New(zap.Level(zapcore.Level(-4)), zap.StacktraceLevel(zapcore.PanicLevel)))

	logger.Info(fmt.Sprintf("start multitenancy-manager with %v allow orphan namespaces option", allowOrphanNamespaces))

	// initialize runtime manager
	runtimeManager, err := setupRuntimeManager(logger)
	if err != nil {
		panic(err)
	}

	// initialize helm client
	helmClient, err := helm.New(helmNamespace, helmTemplatesPath, logger)
	if err != nil {
		panic(err)
	}

	// register project controller
	if err = projectcontroller.Register(runtimeManager, helmClient, logger); err != nil {
		panic(err)
	}

	// register template controller
	if err = templatecontroller.Register(runtimeManager, templatesPath, logger); err != nil {
		panic(err)
	}

	// register namespace controller
	if err = namespacecontroller.Register(runtimeManager, logger); err != nil {
		panic(err)
	}

	// register project webhook
	projectwebhook.Register(runtimeManager, helmClient)

	// register template webhook
	templatewebhook.Register(runtimeManager, serviceAccount)

	if !allowOrphanNamespaces {
		// register namespace webhook
		namespacewebhook.Register(runtimeManager, allowedServiceAccounts)
	}

	// register cluster resource grants: catalog reconciler, binding-status reconcilers and webhooks.
	jsonpathFactory := jsonpath.NewWithCache()
	if err = (&grantcontrollers.ProjectReconciler{
		Client: runtimeManager.GetClient(),
		Mapper: runtimeManager.GetRESTMapper(),
	}).SetupWithManager(runtimeManager); err != nil {
		panic(err)
	}
	if err = (&grantcontrollers.ReferenceReconciler{Client: runtimeManager.GetClient()}).SetupWithManager(runtimeManager); err != nil {
		panic(err)
	}
	if err = (&grantcontrollers.DefinitionReconciler{Client: runtimeManager.GetClient()}).SetupWithManager(runtimeManager); err != nil {
		panic(err)
	}
	// Use the direct (uncached) API reader for the admission webhooks: a cache-backed read lazily
	// starts an informer and blocks on its sync inside the request, which can exceed the webhook
	// deadline and pile up into a queue lock. A direct reader keeps reads bounded.
	grantwebhooks.NewIsGrantedValidator(logger, runtimeManager.GetAPIReader(), runtimeManager.GetRESTMapper(), jsonpathFactory).InstallInto(runtimeManager.GetWebhookServer())
	grantwebhooks.NewDefaultsMutator(logger, runtimeManager.GetAPIReader(), runtimeManager.GetRESTMapper(), jsonpathFactory).InstallInto(runtimeManager.GetWebhookServer())
	grantwebhooks.NewProtectValidator(logger, serviceAccount).InstallInto(runtimeManager.GetWebhookServer())

	// register the project role binding reconcilers
	if err = (&projectrolebindingcontroller.Reconciler{Client: runtimeManager.GetClient()}).SetupWithManager(runtimeManager); err != nil {
		panic(err)
	}
	if err = (&clusterprojectrolebindingcontroller.Reconciler{Client: runtimeManager.GetClient()}).SetupWithManager(runtimeManager); err != nil {
		panic(err)
	}

	// register the project role binding webhooks
	projectrolebindingwebhook.Register(runtimeManager)
	clusterprojectrolebindingwebhook.Register(runtimeManager)

	// start runtime manager
	if err = runtimeManager.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

func setupRuntimeManager(logger logr.Logger) (ctrl.Manager, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		v1alpha1.AddToScheme,
		v1alpha3.AddToScheme,
		grantsv1alpha1.AddToScheme,
		corev1.AddToScheme,
		rbacv1.AddToScheme,
		authorizationv1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			logger.Error(err, "failed to add scheme to runtime manager")
			return nil, err
		}
	}

	opts := manager.Options{
		LeaderElection:          false,
		Scheme:                  scheme,
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
		HealthProbeBindAddress:  ":9090",
		WebhookServer:           webhook.NewServer(webhook.Options{CertDir: "/certs"}),
		Metrics: metrics.Options{
			BindAddress: "0",
		},
	}

	if os.Getenv(haModeEnv) == "true" {
		opts.LeaderElection = true
		opts.LeaderElectionID = controllerName
		opts.LeaderElectionNamespace = helmNamespace
	}

	runtimeManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		logger.Error(err, "unable to create runtime manager")
		return nil, err
	}
	if err = runtimeManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up health check")
		return nil, err
	}
	// Honest readiness: report Ready only once the webhook server is actually serving, instead of an
	// unconditional Ping. Otherwise the pod is Ready while its webhooks cannot answer yet, and the
	// apiserver routes admission to a not-yet-serving replica (webhook timeouts on startup/rollout).
	if err = runtimeManager.AddReadyzCheck("readyz", runtimeManager.GetWebhookServer().StartedChecker()); err != nil {
		logger.Error(err, "unable to set up ready check")
		return nil, err
	}
	return runtimeManager, nil
}
