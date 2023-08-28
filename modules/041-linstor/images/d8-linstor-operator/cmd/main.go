package main

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap/zapcore"
	"linstor-operator/api/v1alpha1"
	"linstor-operator/config"
	"linstor-operator/pkg/controllers"
	kubutils "linstor-operator/pkg/kubeutils"
	"os"
	"os/signal"
	goruntime "runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"syscall"

	v1storage "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	resourcesSchemeFuncs = []func(*apiruntime.Scheme) error{
		v1alpha1.AddToScheme,
		clientgoscheme.AddToScheme,
		extv1.AddToScheme,
	}
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	log := zap.New(zap.Level(zapcore.Level(-1)), zap.UseDevMode(true))
	log.WithName("cmd")

	klog.Info(fmt.Sprintf("Go Version:%s ", goruntime.Version()))
	klog.Info(fmt.Sprintf("OS/Arch:Go OS/Arch:%s/%s ", goruntime.GOOS, goruntime.GOARCH))

	cfgParams, err := config.NewConfig()
	if err != nil {
		klog.Fatalln(err)
	}

	// Create default config Kubernetes client
	kConfig, err := kubutils.KubernetesDefaultConfigCreate()
	if err != nil {
		klog.Fatalln(err)
	}
	log.Info("read Kubernetes config")

	// Setup scheme for all resources
	scheme := runtime.NewScheme()
	for _, f := range resourcesSchemeFuncs {
		err := f(scheme)
		if err != nil {
			log.Error(err, "failed to add to scheme")
			os.Exit(1)
		}
	}
	log.Info("read scheme CR")

	// Create Kubernetes client
	kClient, err := kubutils.CreateKubernetesClient(kConfig, scheme)
	if err != nil {
		klog.Fatalln(err)
	}
	klog.Info("create kubernetes client")

	// Set options for webhook server.
	myWebhookServer := webhook.NewServer(webhook.Options{})
	if cfgParams.CertDir != "" {
		myWebhookServer = webhook.NewServer(webhook.Options{CertDir: cfgParams.CertDir})
	}

	// webhookOpt := webhook.Options{CertDir: "/tmp/linstor-operator-certs"}
	managerOpts := manager.Options{
		LeaderElection:             false,
		LeaderElectionNamespace:    "d8-linstor",
		LeaderElectionID:           "d8-linstor-operator-leader-election-helper",
		LeaderElectionResourceLock: "leases",
		Scheme:                     scheme,
		MetricsBindAddress:         cfgParams.MetricsPort,
		WebhookServer:              myWebhookServer,
	}

	// Create a new Manager to provide shared dependencies and start components
	mgr, err := manager.New(kConfig, managerOpts)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("create kubernetes manager")

	// LinstorNodesReconciler
	// stop := make(chan struct{})
	go func() {
		defer cancel()
		err := controllers.NewLinstorNodesReconciler(ctx, kClient, log, cfgParams.ScanInterval)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// only occurs if the context was cancelled, and it only can be cancelled on SIGINT
				// stop <- struct{}{}
				return
			}
			klog.Error(err, "failed create controller NewLinstorNodesReconciler")
			os.Exit(1)
		}
	}()

	// webHook
	if err := builder.WebhookManagedBy(mgr).
		For(&v1storage.StorageClass{}).
		WithValidator(controllers.NewCSValidator(log)).
		Complete(); err != nil {
		klog.Errorf("error start webhook")
	}

	log.Info("controller LinstorOperator start")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	err = mgr.Start(ctx)
	if err != nil {
		log.Error(err, "error start manager")
		os.Exit(1)
	}

	log.Info("starting the manager")

	// Block waiting signals from OS.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<-ch
	cancel()
	// <-stop
}
