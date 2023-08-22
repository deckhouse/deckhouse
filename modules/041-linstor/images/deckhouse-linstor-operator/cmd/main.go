package main

import (
	"context"
	"fmt"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"linstor-operator/api/v1alpha1"
	"linstor-operator/config"
	"linstor-operator/pkg/controllers"
	kubutils "linstor-operator/pkg/kubeutils"
	"os"
	goruntime "runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	log                  = logf.Log.WithName("cmd")
	resourcesSchemeFuncs = []func(*apiruntime.Scheme) error{
		v1alpha1.AddToScheme,
		clientgoscheme.AddToScheme,
		extv1.AddToScheme,
	}
)

func main() {

	ctx, _ := context.WithCancel(context.Background())

	klog.Info(fmt.Sprintf("Go Version:%s ", goruntime.Version()))
	klog.Info(fmt.Sprintf("OS/Arch:Go OS/Arch:%s/%s ", goruntime.GOOS, goruntime.GOARCH))

	cfgParams, err := config.NewConfig()
	if err != nil {
		klog.Fatalln(err)
	}
	klog.Info("--- storage class ENV ---")
	klog.Info(config.SCStableReplicas+" ", cfgParams.SCStable.Replicas)
	klog.Info(config.SCStableQuorum+" ", cfgParams.SCStable.Quorum)
	klog.Info(config.SCBadReplicas+" ", cfgParams.SCBad.Replicas)
	klog.Info(config.SCBadQuorum+" ", cfgParams.SCBad.Quorum)

	// Create default config Kubernetes client
	kConfig, err := kubutils.KubernetesDefaultConfigCreate()
	if err != nil {
		klog.Fatalln(err)
	}
	klog.Info("read Kubernetes config")

	// Setup scheme for all resources
	scheme := runtime.NewScheme()
	for _, f := range resourcesSchemeFuncs {
		err := f(scheme)
		if err != nil {
			klog.Error("failed to add to scheme", err)
			os.Exit(1)
		}
	}
	klog.Info("read scheme CR")

	managerOpts := manager.Options{
		LeaderElection:             true,
		LeaderElectionNamespace:    "d8-storage-deckhouse-linstor-operator",
		LeaderElectionID:           "d8-storage-deckhouse-linstor-operator-leader-election-helper",
		LeaderElectionResourceLock: "leases",
		Scheme:                     scheme,
		MetricsBindAddress:         cfgParams.MetricsPort,
	}

	// Create a new Manager to provide shared dependencies and start components
	mgr, err := manager.New(kConfig, managerOpts)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	klog.Info("create kubernetes manager")

	if _, err := controllers.NewLinstorOperator(ctx, mgr, log); err != nil {
		klog.Error("failed create controller NewLinstorOperator", err)
		os.Exit(1)
	}

	klog.Info("controller LinstorOperator start")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	err = mgr.Start(ctx)
	if err != nil {
		klog.Error(err, "error start manager")
		os.Exit(1)
	}

	klog.Info("starting the manager")
}
