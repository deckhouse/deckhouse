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

package main

import (
	"os"

	kruiseappsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ingressnginxv1 "manager/src/api/v1"
	"manager/src/internal"
)

const (
	leaderElectionID        = "ingress-nginx-manager.deckhouse.io"
	leaderElectionNamespace = "d8-ingress-nginx"
	watchNamespace          = "d8-ingress-nginx"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("ingress-nginx-manager")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kruiseappsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ingressnginxv1.AddToScheme(scheme))

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		LeaderElection:          true,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectionNamespace,
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		HealthProbeBindAddress: ":8081",
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&ingressnginxv1.IngressNginxController{}: {},
				&appsv1.DaemonSet{}: {
					Namespaces: map[string]cache.Config{
						watchNamespace: {},
					},
				},
				&kruiseappsv1alpha1.DaemonSet{}: {
					Namespaces: map[string]cache.Config{
						watchNamespace: {},
					},
				},
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						watchNamespace: {},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "could not add health check")
		os.Exit(1)
	}

	if err := manager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "could not add ready check")
		os.Exit(1)
	}

	internal.SetupController(manager, log.WithName("controller"))

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}
