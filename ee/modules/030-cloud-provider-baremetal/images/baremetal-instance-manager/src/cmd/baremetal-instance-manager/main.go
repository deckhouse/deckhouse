/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
	var bmcProbeTimeout time.Duration
	flag.StringVar(&targetNamespace, "target-namespace", "d8-cloud-instance-manager", "namespace where BareMetalInstance and generated BareMetalHost resources are stored")
	flag.DurationVar(&bmcProbeTimeout, "bmc-probe-timeout", 15*time.Second, "timeout for a single BMC protocol probe")
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
	instance.SetGroupVersionKind(bareMetalInstanceGVK)
	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)

	r := &reconciler{
		Client:          mgr.GetClient(),
		targetNamespace: targetNamespace,
		resolver:        newNetworkBMCResolver(bmcProbeTimeout),
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
