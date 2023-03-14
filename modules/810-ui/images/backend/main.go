package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/julienschmidt/httprouter"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	// for local kubeconfig
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type appConfig struct {
	listenPort   string
	resyncPeriod time.Duration
	kubeConfig   *rest.Config
}

func main() {
	appConfig := getConfig()

	// Init factory for informers for well-known types
	clientset, err := kubernetes.NewForConfig(appConfig.kubeConfig)
	if err != nil {
		klog.Fatal(fmt.Errorf("creating clientset: %v", err.Error()))
	}
	factory := informers.NewSharedInformerFactory(clientset, appConfig.resyncPeriod)
	defer factory.Shutdown()

	// Init factory for informers for custom types
	dynClient, err := dynamic.NewForConfig(appConfig.kubeConfig)
	if err != nil {
		klog.Fatal(fmt.Errorf("creating dynamic client: %v", err.Error()))
	}
	dynFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, appConfig.resyncPeriod)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := httprouter.New()
	handler, err := initHandlers(ctx, router, clientset, factory, dynClient, dynFactory)
	if err != nil {
		klog.Fatal(fmt.Errorf("initializing handlers: %v", err.Error()))
	}

	router.GET("/healthz", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) { w.WriteHeader(200) })

	var inSync atomic.Bool
	router.GET("/readyz", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if inSync.Load() {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(500)
	})

	errc := make(chan error, 1)
	go func() {
		// Start informers all at once after we have inited them in initHandlers func
		factory.Start(ctx.Done()) // Start processing these informers.
		klog.Info("Started informers.")
		// Wait for cache sync
		klog.Info("Waiting for initial sync of informers.")
		synced := factory.WaitForCacheSync(ctx.Done())
		for v, ok := range synced {
			if !ok {
				errc <- fmt.Errorf("caches failed to sync: %v", v)
			}
		}

		// Start dynamic informers all at once after we have inited them in initHandlers func
		dynFactory.Start(ctx.Done())
		klog.Info("Started dynamic informers.")
		// Wait for cache sync for dynamic informers
		klog.Info("Waiting for initial sync of dynamic informers.")
		dynSynced := dynFactory.WaitForCacheSync(ctx.Done())
		for v, ok := range dynSynced {
			if !ok {
				errc <- fmt.Errorf("dynamic caches failed to sync: %v", v)
			}
		}

		inSync.Store(true)
	}()

	klog.Info("Listening :" + appConfig.listenPort)

	srv := &http.Server{
		Handler: handler,
		Addr:    ":" + appConfig.listenPort,
	}

	go func() {
		errc <- srv.ListenAndServe()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		klog.Errorf("failed: %v", err)
	case sig := <-sigs:
		klog.Infof("terminating: %v", sig)
	}

	shutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		klog.Errorf("shutdown: %v", err)
	}
}

func getConfig() *appConfig {
	flagSet := flag.NewFlagSet("dashboard", flag.ExitOnError)
	klog.InitFlags(flagSet)

	port := flagSet.String("port", "8999", "port to listen on")
	resyncPeriod := flagSet.Duration("resyncPeriod-period", 10*time.Minute, "informers resyncPeriod period")
	// create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.V(10).Info("error getting in-cluster config, falling back to local config")
		// create local config
		if !errors.Is(err, rest.ErrNotInCluster) {
			// the only recognized error
			klog.Fatal(fmt.Errorf("getting kube client config: %v", err.Error()))
		}

		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flagSet.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flagSet.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}

		err := flagSet.Parse(os.Args[1:])
		if err != nil {
			klog.Fatal(fmt.Errorf("parsing flags: %v", err.Error()))
		}

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			klog.Fatal(fmt.Errorf("building kube client config: %v", err.Error()))
		}

	} else {
		err := flagSet.Parse(os.Args[1:])
		if err != nil {
			klog.Fatal(fmt.Errorf("parsing flags: %v", err.Error()))
		}
	}

	return &appConfig{
		listenPort:   *port,
		resyncPeriod: *resyncPeriod,
		kubeConfig:   config,
	}
}
