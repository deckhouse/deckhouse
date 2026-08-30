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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

func main() {
	var targetNamespace string
	flag.StringVar(&targetNamespace, "target-namespace", "d8-cloud-instance-manager", "namespace where Metal3Instance and generated BareMetalHost resources are stored")
	flag.Parse()

	zapLogger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger: %v\n", err)
		os.Exit(1)
	}
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		fmt.Fprintf(os.Stderr, "register core scheme: %v\n", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{
			targetNamespace: {},
		}},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create manager: %v\n", err)
		os.Exit(1)
	}

	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(metal3InstanceGVK)
	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)

	r := &reconciler{
		Client:          mgr.GetClient(),
		targetNamespace: targetNamespace,
		resolver:        newNetworkBMCResolver(15 * time.Second),
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(instance).
		Watches(bmh, handler.EnqueueRequestsFromMapFunc(r.bareMetalHostToInstance)).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretToInstances)).
		Complete(r); err != nil {
		fmt.Fprintf(os.Stderr, "create controller: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "run manager: %v\n", err)
		os.Exit(1)
	}
}
