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

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	projectcontroller "controller/pkg/controller/project"
	templatecontroller "controller/pkg/controller/template"
	"controller/pkg/helm"
	namespacewebhook "controller/pkg/webhook/namespace"
	projectwebhook "controller/pkg/webhook/project"
	templatewebhook "controller/pkg/webhook/template"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	// path to helm templates
	templatesPath = "templates"
	// path to default project templates
	defaultPath = "default"
	// helm release namespace
	helmNamespace = "d8-multitenancy-manager"
	// controller service account
	serviceAccount = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	// list of service accounts allowed to create namespaces when allowNamespacesWithoutProjects is set to false
	nsCreateAllowedServiceAccounts = []string{serviceAccount, "system:serviceaccount:d8-system:deckhouse", "system:serviceaccount:d8-upmeter:upmeter-agent"}
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
	log := ctrl.Log.WithName(controllerName)
	ctrllog.SetLogger(zap.New(zap.Level(zapcore.Level(-4)), zap.StacktraceLevel(zapcore.PanicLevel)))

	log.Info(fmt.Sprintf("starting multitenancy-manager with %v allow orphan namespaces option", allowOrphanNamespaces))

	// initialize runtime manager
	runtimeManager, err := setupRuntimeManager(log)
	if err != nil {
		panic(err)
	}

	// initialize helm client
	helmClient, err := helm.New(helmNamespace, templatesPath, log)
	if err != nil {
		panic(err)
	}

	// register project controller
	if err = projectcontroller.Register(runtimeManager, helmClient, log); err != nil {
		panic(err)
	}

	// register template controller
	if err = templatecontroller.Register(runtimeManager, defaultPath, log); err != nil {
		panic(err)
	}

	// register project webhook
	projectwebhook.Register(runtimeManager, helmClient)

	// register template webhook
	templatewebhook.Register(runtimeManager, serviceAccount)

	if !allowOrphanNamespaces {
		// register namespace webhook
		namespacewebhook.Register(runtimeManager, nsCreateAllowedServiceAccounts)
	}

	// start runtime manager
	if err = runtimeManager.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

func setupRuntimeManager(log logr.Logger) (ctrl.Manager, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
		corev1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			log.Error(err, "failed to add scheme to runtime manager")
			return nil, err
		}
	}

	managerOpts := manager.Options{
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
		managerOpts.LeaderElection = true
		managerOpts.LeaderElectionID = controllerName
		managerOpts.LeaderElectionNamespace = helmNamespace
	}

	runtimeManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOpts)
	if err != nil {
		log.Error(err, "unable to create runtime manager")
		return nil, err
	}
	if err = runtimeManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		return nil, err
	}
	if err = runtimeManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		return nil, err
	}
	return runtimeManager, nil
}
